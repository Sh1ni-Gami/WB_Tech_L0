package data_base

import (
	"context"
	"time"

	"log/slog"

	"github.com/Sh1ni-Gami/WB_Tech_L0/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBService интерфейс для работы с базой данных.
type DBService interface {
	AddOrder(ctx context.Context, order *model.OrderDetails) error
	GetOrder(ctx context.Context, orderUID string) (*model.OrderDetails, error)
	GetRecentOrderIDs(ctx context.Context, limit int) ([]string, error)
}

type dbService struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New создает экземпляр DBService.
func New(connString string, logger *slog.Logger) (DBService, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}

	return &dbService{
		pool:   pool,
		logger: logger,
	}, nil
}

// AddOrder добавляет заказ в базу данных.
func (s *dbService) AddOrder(ctx context.Context, order *model.OrderDetails) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Добавление AddressDetails
	var deliveryID int
	err = tx.QueryRow(ctx,
		`INSERT INTO delivery (name, phone, zip, city, address, region, email)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		order.Address.FullName, order.Address.Phone, order.Address.ZipCode,
		order.Address.City, order.Address.Street, order.Address.Region, order.Address.Email).
		Scan(&deliveryID)
	if err != nil {
		s.logger.Error("Failed to insert delivery", slog.Any("error", err))
		return err
	}

	// Добавление PaymentDetails
	_, err = tx.Exec(ctx,
		`INSERT INTO payment (transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		order.Payment.TransactionID, order.Payment.RequestID, order.Payment.Currency,
		order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDate,
		order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.TotalGoods, order.Payment.CustomFee)
	if err != nil {
		s.logger.Error("Failed to insert payment", slog.Any("error", err))
		return err
	}

	// Добавление OrderDetails
	_, err = tx.Exec(ctx,
		`INSERT INTO orders (order_uid, track_number, entry, delivery_id, payment_id, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		order.OrderID, order.TrackingNumber, order.EntryPoint, deliveryID, order.Payment.TransactionID,
		order.Locale, order.Signature, order.CustomerID,
		order.DeliveryService, order.ShardKey, order.SMID, time.Time(order.CreationTimestamp), order.OutOfShard)
	if err != nil {
		s.logger.Error("Failed to insert order", slog.Any("error", err))
		return err
	}

	// Добавление ProductItem
	for _, item := range order.Products {
		_, err = tx.Exec(ctx,
			`INSERT INTO items (chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			item.ChartID, item.TrackingNum, item.Price, item.RID, item.Name,
			item.Discount, item.Size, item.TotalPrice, item.ProductID, item.Brand, item.Status)
		if err != nil {
			s.logger.Error("Failed to insert item", slog.Any("error", err))
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		s.logger.Error("Failed to commit transaction", slog.Any("error", err))
		return err
	}

	return nil
}

// GetOrder получает заказ по UID.
func (s *dbService) GetOrder(ctx context.Context, orderUID string) (*model.OrderDetails, error) {
	row := s.pool.QueryRow(ctx, `SELECT * FROM orders WHERE order_uid = $1`, orderUID)
	var orderRecord struct {
		OrderUID        string
		TrackNumber     string
		Entry           string
		DeliveryID      int
		PaymentID       string
		Locale          string
		InternalSig     string
		CustomerID      string
		DeliveryService string
		ShardKey        string
		SmID            int
		DateCreated     time.Time
		OofShard        string
	}

	if err := row.Scan(&orderRecord.OrderUID, &orderRecord.TrackNumber, &orderRecord.Entry, &orderRecord.DeliveryID,
		&orderRecord.PaymentID, &orderRecord.Locale, &orderRecord.InternalSig, &orderRecord.CustomerID,
		&orderRecord.DeliveryService, &orderRecord.ShardKey, &orderRecord.SmID, &orderRecord.DateCreated,
		&orderRecord.OofShard); err != nil {
		s.logger.Error("Failed to fetch order", slog.String("order_uid", orderUID), slog.Any("error", err))
		return nil, err
	}

	// Fetch delivery details
	var deliveryRecord model.AddressDetails
	err := s.pool.QueryRow(ctx, `SELECT name, phone, zip, city, address, region, email FROM delivery WHERE id = $1`, orderRecord.DeliveryID).
		Scan(&deliveryRecord.FullName, &deliveryRecord.Phone, &deliveryRecord.ZipCode, &deliveryRecord.City,
			&deliveryRecord.Street, &deliveryRecord.Region, &deliveryRecord.Email)
	if err != nil {
		s.logger.Error("Failed to fetch delivery", slog.Any("error", err))
		return nil, err
	}

	// Fetch payment details
	var paymentRecord model.PaymentDetails
	err = s.pool.QueryRow(ctx, `SELECT transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee FROM payment WHERE transaction = $1`, orderRecord.PaymentID).
		Scan(&paymentRecord.TransactionID, &paymentRecord.RequestID, &paymentRecord.Currency, &paymentRecord.Provider,
			&paymentRecord.Amount, &paymentRecord.PaymentDate, &paymentRecord.Bank, &paymentRecord.DeliveryCost,
			&paymentRecord.TotalGoods, &paymentRecord.CustomFee)
	if err != nil {
		s.logger.Error("Failed to fetch payment", slog.Any("error", err))
		return nil, err
	}

	// Fetch items
	rows, err := s.pool.Query(ctx, `SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status FROM items WHERE chrt_id IN (SELECT chrt_id FROM order_item_conn WHERE order_uid = $1)`, orderRecord.OrderUID)
	if err != nil {
		s.logger.Error("Failed to fetch items", slog.Any("error", err))
		return nil, err
	}
	defer rows.Close()

	var items []model.ProductItem
	for rows.Next() {
		var item model.ProductItem
		if err := rows.Scan(&item.ChartID, &item.TrackingNum, &item.Price, &item.RID, &item.Name, &item.Discount,
			&item.Size, &item.TotalPrice, &item.ProductID, &item.Brand, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	// Return full order
	return &model.OrderDetails{
		OrderID:           orderRecord.OrderUID,
		TrackingNumber:    orderRecord.TrackNumber,
		EntryPoint:        orderRecord.Entry,
		Address:           deliveryRecord,
		Payment:           paymentRecord,
		Products:          items,
		Locale:            orderRecord.Locale,
		Signature:         orderRecord.InternalSig,
		CustomerID:        orderRecord.CustomerID,
		DeliveryService:   orderRecord.DeliveryService,
		ShardKey:          orderRecord.ShardKey,
		SMID:              orderRecord.SmID,
		CreationTimestamp: model.ISO8601Time(orderRecord.DateCreated),
		OutOfShard:        orderRecord.OofShard,
	}, nil
}

// GetRecentOrderIDs возвращает последние `limit` заказов.
func (s *dbService) GetRecentOrderIDs(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT order_uid FROM orders ORDER BY date_created DESC LIMIT $1`, limit)
	if err != nil {
		s.logger.Error("Failed to fetch recent order IDs", slog.Any("error", err))
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}
