package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/Sh1ni-Gami/WB_Tech_L0/model"
	"github.com/segmentio/kafka-go"
)

// Store интерфейс для взаимодействия с хранилищем.
type Store interface {
	AddOrder(order *model.OrderDetails) error
	GetOrder(orderUID string) (*model.OrderDetails, error)
}

// KafkaService интерфейс для работы с Kafka.
type KafkaService interface {
	StartListening(ctx context.Context)
	SendOrder(ctx context.Context, order *model.OrderDetails) error
}

type kafkaService struct {
	reader    *kafka.Reader
	writer    *kafka.Writer
	store     Store
	logger    *slog.Logger
	topic     string
	partition int
}

// NewKafkaService создает новый экземпляр KafkaService.
func NewKafkaService(topic, brokerURL string, partition string, logger *slog.Logger, store Store) (KafkaService, error) {
	part, err := strconv.Atoi(partition)
	if err != nil {
		return nil, errors.New("invalid partition: must be an integer")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Topic:     topic,
		Partition: part,
		Brokers:   []string{brokerURL},
	})

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokerURL),
		Topic:        topic,
		RequiredAcks: kafka.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}

	return &kafkaService{
		reader:    reader,
		writer:    writer,
		store:     store,
		logger:    logger,
		topic:     topic,
		partition: part,
	}, nil
}

// StartListening начинает прослушивание Kafka и обработку сообщений.
func (k *kafkaService) StartListening(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				k.logger.Info("Kafka listener shutting down gracefully")
				if err := k.reader.Close(); err != nil {
					k.logger.Error("Error closing Kafka reader", slog.Any("error", err))
				}
				return
			default:
				msg, err := k.reader.ReadMessage(ctx)
				if err != nil {
					k.logger.Warn("Failed to read message from Kafka", slog.Any("error", err))
					continue
				}

				k.logger.Debug("Message received from Kafka", slog.String("topic", msg.Topic), slog.Int("partition", msg.Partition))

				order, err := k.decodeOrder(msg.Value)
				if err != nil {
					k.logger.Error("Failed to decode order message", slog.Any("error", err))
					continue
				}

				if err := k.store.AddOrder(order); err != nil {
					k.logger.Error("Failed to save order to store", slog.Any("error", err))
					continue
				}

				k.logger.Info("Order processed successfully", slog.String("orderID", order.OrderID))
			}
		}
	}()
}

// SendOrder отправляет заказ в Kafka.
func (k *kafkaService) SendOrder(ctx context.Context, order *model.OrderDetails) error {
	orderBytes, err := json.Marshal(order)
	if err != nil {
		return errors.New("failed to serialize order to JSON")
	}

	err = k.writer.WriteMessages(ctx, kafka.Message{
		Value: orderBytes,
	})
	if err != nil {
		k.logger.Error("Failed to send order to Kafka", slog.Any("error", err))
		return err
	}

	k.logger.Info("Order sent successfully", slog.String("orderID", order.OrderID), slog.String("topic", k.topic))
	return nil
}

// decodeOrder декодирует сообщение Kafka в структуру OrderDetails.
func (k *kafkaService) decodeOrder(data []byte) (*model.OrderDetails, error) {
	var order model.OrderDetails
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, errors.New("invalid order format in Kafka message")
	}
	return &order, nil
}

// Utility function: getEnv возвращает значение переменной окружения или значение по умолчанию.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
