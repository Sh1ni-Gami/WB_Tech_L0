package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sh1ni-Gami/WB_Tech_L0/model"
)

// Store интерфейс для взаимодействия с хранилищем.
type Store interface {
	AddOrder(order *model.OrderDetails) error
	GetOrder(orderUID string) (*model.OrderDetails, error)
}

// HTTPTransport интерфейс для работы с HTTP-сервером.
type HTTPTransport interface {
	Start(ctx context.Context, addr string) error
}

// httpTransport реализует HTTPTransport.
type httpTransport struct {
	store  Store
	logger *slog.Logger
	server *http.Server
}

// NewHTTPTransport создает экземпляр HTTPTransport.
func NewHTTPTransport(store Store, logger *slog.Logger) HTTPTransport {
	return &httpTransport{
		store:  store,
		logger: logger,
	}
}

// Start запускает HTTP-сервер с поддержкой graceful shutdown.
func (t *httpTransport) Start(ctx context.Context, addr string) error {
	router := http.NewServeMux()
	router.HandleFunc("/api/v1/order", t.orderHandler)
	router.HandleFunc("/", t.interfaceHandler)

	t.server = &http.Server{
		Addr:        addr,
		Handler:     router,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	// Обработка сигнала завершения для graceful shutdown
	go t.listenForShutdown(ctx)

	t.logger.Info("HTTP server starting", slog.String("address", addr))
	if err := t.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.logger.Error("HTTP server failed", slog.Any("error", err))
		return err
	}

	return nil
}

// listenForShutdown ожидает сигналы завершения и корректно завершает работу сервера.
func (t *httpTransport) listenForShutdown(ctx context.Context) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case <-stop:
	}

	t.logger.Info("Shutting down HTTP server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := t.server.Shutdown(ctx); err != nil {
		t.logger.Error("Failed to shut down HTTP server gracefully", slog.Any("error", err))
	} else {
		t.logger.Info("HTTP server shut down successfully")
	}
}

// orderHandler обрабатывает запросы для получения данных заказа.
func (t *httpTransport) orderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	orderUID := r.URL.Query().Get("order_uid")
	if orderUID == "" {
		http.Error(w, "Missing order UID", http.StatusBadRequest)
		return
	}

	order, err := t.store.GetOrder(orderUID)
	if err != nil {
		t.logger.Error("Failed to fetch order", slog.String("orderUID", orderUID), slog.Any("error", err))
		http.Error(w, fmt.Sprintf("Error fetching order: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		t.logger.Error("Failed to encode order to JSON", slog.Any("error", err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// interfaceHandler возвращает HTML-страницу для пользовательского интерфейса.
func (t *httpTransport) interfaceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	htmlFile, err := os.ReadFile("frontend/form.html")
	if err != nil {
		t.logger.Error("Failed to read HTML file", slog.Any("error", err))
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(htmlFile); err != nil {
		t.logger.Error("Failed to write HTML response", slog.Any("error", err))
	}
}
