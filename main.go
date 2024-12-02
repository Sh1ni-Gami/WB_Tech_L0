package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	ristrettocache "github.com/Sh1ni-Gami/WB_Tech_L0/caching"
	"github.com/Sh1ni-Gami/WB_Tech_L0/data_base"
	"github.com/Sh1ni-Gami/WB_Tech_L0/kafka"
	httptransport "github.com/Sh1ni-Gami/WB_Tech_L0/transport"
	"github.com/joho/godotenv"
)

const cacheSize = 1024

// App структура для управления зависимостями приложения.
type App struct {
	Logger     *slog.Logger
	DB         data_base.DBService
	Cache      ristrettocache.CacheService
	Kafka      kafka.KafkaService
	Transport  httptransport.HTTPTransport
	Ctx        context.Context
	CancelFunc context.CancelFunc
}

// NewApp создает новое приложение, инициализируя все зависимости.
func NewApp() (*App, error) {
	// Создаем контекст и логгер.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Загружаем переменные окружения.
	if err := godotenv.Load(); err != nil {
		logger.Warn("Error loading .env file, using system environment variables")
	}

	// Инициализируем базу данных.
	dbConn, err := initDatabase(logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Инициализируем кэш.
	cache, err := ristrettocache.NewCacheService(logger, cacheSize, dbConn)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Инициализируем Kafka.
	kafkaService, err := initKafka(logger, cache)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize Kafka: %w", err)
	}

	// Инициализируем HTTP-транспорт.
	httpTransport := httptransport.NewHTTPTransport(cache, logger)

	return &App{
		Logger:     logger,
		DB:         dbConn,
		Cache:      cache,
		Kafka:      kafkaService,
		Transport:  httpTransport,
		Ctx:        ctx,
		CancelFunc: cancel,
	}, nil
}

// initDatabase инициализирует подключение к базе данных.
func initDatabase(logger *slog.Logger) (data_base.DBService, error) {
	user := getEnv("POSTGRES_USER", "postgres")
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD is not set")
	}

	url := getEnv("POSTGRES_URL", "localhost:5432")
	dbConnString := fmt.Sprintf("postgres://%s:%s@%s/wb_tech", user, password, url)

	dbConn, err := data_base.New(dbConnString, logger)
	if err != nil {
		logger.Error("Failed to connect to the database", slog.Any("error", err))
		return nil, err
	}

	logger.Info("Database connection established")
	return dbConn, nil
}

// initKafka инициализирует подключение к Kafka.
func initKafka(logger *slog.Logger, cache ristrettocache.CacheService) (kafka.KafkaService, error) {
	partition := getEnv("KAFKA_PARTITION", "0")
	topic := getEnv("KAFKA_TOPIC", "wb-topic")
	url := getEnv("KAFKA_URL", "localhost:9092")

	kafkaService, err := kafka.NewKafkaService(topic, url, partition, logger, cache)
	if err != nil {
		logger.Error("Failed to connect to Kafka", slog.Any("error", err))
		return nil, err
	}

	logger.Info("Kafka service initialized")
	return kafkaService, nil
}

// getEnv возвращает значение переменной окружения или значение по умолчанию.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// Создаём приложение.
	app, err := NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize app: %v\n", err)
		os.Exit(1)
	}
	defer app.CancelFunc()

	// Запуск Kafka listener.
	go func() {
		app.Kafka.StartListening(app.Ctx)
		app.Logger.Info("Kafka listener started")
	}()

	// Запуск HTTP-сервера.
	go func() {
		if err := app.Transport.Start(app.Ctx, ":8080"); err != nil {
			app.Logger.Error("HTTP server failed", slog.Any("error", err))
			app.CancelFunc()
		}
		app.Logger.Info("HTTP server started on :8080")
	}()

	// Ожидание завершения.
	<-app.Ctx.Done()
	time.Sleep(1 * time.Second) // Ожидание завершения всех операций.
	app.Logger.Info("Application shut down gracefully")
}
