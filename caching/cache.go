package ristrettocache

import (
	"log/slog"
	"sync"

	"github.com/Sh1ni-Gami/WB_Tech_L0/model"
	"github.com/dgraph-io/ristretto"
)

// CacheService интерфейс для работы с кэшем.
type CacheService interface {
	AddOrder(order *model.OrderDetails) error
	GetOrder(orderUID string) (*model.OrderDetails, error)
}

// DBService интерфейс для взаимодействия с базой данных.
type DBService interface {
	AddOrder(order *model.OrderDetails) error
	GetOrder(orderUID string) (*model.OrderDetails, error)
	GetRecentOrderIDs(limit int) ([]string, error)
}

// cacheService реализует CacheService.
type cacheService struct {
	cache   *ristretto.Cache
	db      DBService
	logger  *slog.Logger
	maxSize int
}

// NewCacheService создает новый сервис с поддержкой Ristretto.
func NewCacheService(logger *slog.Logger, cacheSize int, db DBService) (CacheService, error) {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(cacheSize) * 10, // NumCounters рекомендуется как 10x от MaxCost
		MaxCost:     int64(cacheSize),
		BufferItems: 64, // Количество буферных элементов для асинхронной записи
	})
	if err != nil {
		return nil, err
	}

	service := &cacheService{
		cache:   ristrettoCache,
		db:      db,
		logger:  logger,
		maxSize: cacheSize,
	}

	// Инициализация кэша
	if err := service.loadCache(); err != nil {
		return nil, err
	}

	return service, nil
}

// loadCache загружает последние заказы из базы в кэш.
func (s *cacheService) loadCache() error {
	s.logger.Info("Initializing cache with recent orders...")
	orderIDs, err := s.db.GetRecentOrderIDs(s.maxSize)
	if err != nil {
		s.logger.Error("Failed to load recent orders from DB", slog.Any("error", err))
		return err
	}

	var wg sync.WaitGroup
	for _, orderID := range orderIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			order, err := s.db.GetOrder(id)
			if err != nil {
				s.logger.Warn("Failed to fetch order during cache init", slog.String("orderID", id), slog.Any("error", err))
				return
			}

			// Добавляем в кэш
			if ok := s.cache.Set(id, order, 1); ok {
				s.logger.Debug("Order added to cache", slog.String("orderID", id))
			}
			s.cache.Wait()
		}(orderID)
	}

	wg.Wait()
	s.logger.Info("Cache initialization complete")
	return nil
}

// AddOrder добавляет заказ в кэш и базу данных.
func (s *cacheService) AddOrder(order *model.OrderDetails) error {
	s.logger.Debug("Adding order to cache", slog.String("orderID", order.OrderID))
	s.cache.Set(order.OrderID, order, 1)
	s.cache.Wait()

	if err := s.db.AddOrder(order); err != nil {
		s.logger.Error("Failed to add order to DB", slog.String("orderID", order.OrderID), slog.Any("error", err))
		return err
	}

	s.logger.Info("Order added successfully", slog.String("orderID", order.OrderID))
	return nil
}

// GetOrder получает заказ из кэша или базы данных.
func (s *cacheService) GetOrder(orderUID string) (*model.OrderDetails, error) {
	// Сначала пытаемся найти заказ в кэше
	order, found := s.getFromCache(orderUID)
	if found {
		s.logger.Debug("Cache hit", slog.String("orderID", orderUID))
		return order, nil
	}

	s.logger.Debug("Cache miss", slog.String("orderID", orderUID))

	// Если в кэше нет, загружаем из базы
	order, err := s.db.GetOrder(orderUID)
	if err != nil {
		s.logger.Error("Failed to fetch order from DB", slog.String("orderID", orderUID), slog.Any("error", err))
		return nil, err
	}

	// Сохраняем в кэш для дальнейшего использования
	s.addToCache(orderUID, order)
	return order, nil
}

// getFromCache пытается получить заказ из кэша.
func (s *cacheService) getFromCache(orderUID string) (*model.OrderDetails, bool) {
	item, found := s.cache.Get(orderUID)
	if !found {
		return nil, false
	}

	order, ok := item.(*model.OrderDetails)
	if !ok {
		s.logger.Warn("Cache contains invalid data type", slog.String("orderID", orderUID))
		return nil, false
	}

	return order, true
}

// addToCache добавляет заказ в кэш.
func (s *cacheService) addToCache(orderUID string, order *model.OrderDetails) {
	ok := s.cache.Set(orderUID, order, 1)
	s.cache.Wait()
	if ok {
		s.logger.Debug("Order added to cache", slog.String("orderID", orderUID))
	}
}
