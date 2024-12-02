// Пакет для внесения заказа в БД
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"log/slog"

	"github.com/Sh1ni-Gami/WB_Tech_L0/kafka"
	"github.com/Sh1ni-Gami/WB_Tech_L0/model"
	"github.com/joho/godotenv"
)

// getEnv возвращает значение переменной окружения или значение по умолчанию.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// Загружаем переменные окружения.
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v\n", err)
	}

	// Создаем контекст с тайм-аутом.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Получаем параметры Kafka из переменных окружения.
	kafkaPartition, err := strconv.Atoi(getEnv("KAFKA_PARTITION", "0"))
	if err != nil {
		log.Fatalf("Invalid Kafka partition: %v\n", err)
	}
	kafkaTopic := getEnv("KAFKA_TOPIC", "wb-topic")
	kafkaURL := getEnv("KAFKA_URL", "localhost:9094")

	// Логгер.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Создаем Kafka сервис.
	kafkaService, err := kafka.NewKafkaService(kafkaTopic, kafkaURL, strconv.Itoa(kafkaPartition), logger, nil)
	if err != nil {
		log.Fatalf("Failed to create Kafka service: %v\n", err)
	}

	// Создаем данные для отправки.
	order := &model.OrderDetails{
		OrderID:        "b563feb7b2b84b6test",
		TrackingNumber: "WBILMTESTTRACK",
		EntryPoint:     "WBIL",
		Address: model.AddressDetails{
			FullName: "Test Testov",
			Phone:    "+9720000000",
			ZipCode:  "2639809",
			City:     "Kiryat Mozkin",
			Street:   "Ploshad Mira 15",
			Region:   "Kraiot",
			Email:    "test@gmail.com",
		},
		Payment: model.PaymentDetails{
			TransactionID: "b563feb7b2b84b6test",
			RequestID:     "",
			Currency:      "USD",
			Provider:      "wbpay",
			Amount:        1817,
			PaymentDate:   1637907727,
			Bank:          "alpha",
			DeliveryCost:  1500,
			TotalGoods:    317,
			CustomFee:     0,
		},
		Products: []model.ProductItem{
			{
				ChartID:     9934930,
				TrackingNum: "WBILMTESTTRACK",
				Price:       453,
				RID:         "ab4219087a764ae0btest",
				Name:        "Mascaras",
				Discount:    30,
				Size:        "0",
				TotalPrice:  317,
				ProductID:   2389212,
				Brand:       "Vivienne Sabo",
				Status:      202,
			},
		},
		Locale:            "en",
		Signature:         "",
		CustomerID:        "test",
		DeliveryService:   "meest",
		ShardKey:          "9",
		SMID:              99,
		CreationTimestamp: model.ISO8601Time(time.Date(2021, time.November, 26, 6, 22, 19, 0, time.UTC)),
		OutOfShard:        "1",
	}

	// Отправляем заказ в Kafka.
	err = kafkaService.SendOrder(ctx, order)
	if err != nil {
		log.Fatalf("Failed to send order to Kafka: %v\n", err)
	}

	logger.Info("Order sent successfully", slog.String("orderID", order.OrderID))
}
