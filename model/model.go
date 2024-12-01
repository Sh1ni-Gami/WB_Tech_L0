package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-faker/faker/v4"
)

type AddressDetails struct {
	FullName string `json:"name" faker:"name"`
	Phone    string `json:"phone" faker:"phone_number"`
	ZipCode  string `json:"zip" faker:"word"`
	City     string `json:"city" faker:"word"`
	Street   string `json:"address" faker:"real_address"`
	Region   string `json:"region" faker:"word"`
	Email    string `json:"email" faker:"email"`
}

type PaymentDetails struct {
	TransactionID string `json:"transaction" faker:"uuid_hyphenated"`
	RequestID     string `json:"request_id" faker:"uuid_hyphenated"`
	Currency      string `json:"currency" faker:"word"`
	Provider      string `json:"provider" faker:"word"`
	Amount        int    `json:"amount"`
	PaymentDate   int    `json:"payment_dt"`
	Bank          string `json:"bank" faker:"word"`
	DeliveryCost  int    `json:"delivery_cost"`
	TotalGoods    int    `json:"goods_total"`
	CustomFee     int    `json:"custom_fee"`
}

type ProductItem struct {
	ChartID     int    `json:"chrt_id"`
	TrackingNum string `json:"track_number" faker:"uuid_hyphenated"`
	Price       int    `json:"price"`
	RID         string `json:"rid"`
	Name        string `json:"name" faker:"word"`
	Discount    int    `json:"sale"`
	Size        string `json:"size" faker:"word"`
	TotalPrice  int    `json:"total_price"`
	ProductID   int    `json:"nm_id"`
	Brand       string `json:"brand" faker:"word"`
	Status      int    `json:"status"`
}

type ISO8601Time time.Time

func (t *ISO8601Time) UnmarshalJSON(data []byte) error {
	str := string(bytes.Trim(data, `"`))
	parsedTime, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return fmt.Errorf("invalid time format: %s", str)
	}
	*t = ISO8601Time(parsedTime)
	return nil
}

func (t ISO8601Time) MarshalJSON() ([]byte, error) {
	formatted := fmt.Sprintf("\"%s\"", time.Time(t).Format(time.RFC3339))
	return []byte(formatted), nil
}

type OrderDetails struct {
	OrderID           string         `json:"order_uid" faker:"uuid_hyphenated"`
	TrackingNumber    string         `json:"track_number" faker:"uuid_hyphenated"`
	EntryPoint        string         `json:"entry"`
	Address           AddressDetails `json:"delivery"`
	Payment           PaymentDetails `json:"payment"`
	Products          []ProductItem  `json:"items"`
	Locale            string         `json:"locale"`
	Signature         string         `json:"internal_signature"`
	CustomerID        string         `json:"customer_id" faker:"uuid_hyphenated"`
	DeliveryService   string         `json:"delivery_service" faker:"word"`
	ShardKey          string         `json:"shardkey"`
	SMID              int            `json:"sm_id"`
	CreationTimestamp ISO8601Time    `json:"date_created"`
	OutOfShard        string         `json:"oof_shard"`
}

func NewFakeOrder(maxItems int) (*OrderDetails, error) {
	order := OrderDetails{}
	if err := faker.FakeData(&order); err != nil {
		return nil, fmt.Errorf("failed to generate fake data: %w", err)
	}
	if len(order.Products) > maxItems {
		order.Products = order.Products[:maxItems]
	}
	order.CreationTimestamp = ISO8601Time(time.Now())
	return &order, nil
}

func ParseOrder(data []byte, maxItems int) (*OrderDetails, error) {
	var order OrderDetails
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&order); err != nil {
		return nil, fmt.Errorf("invalid JSON structure: %w", err)
	}
	if len(order.Products) > maxItems {
		return nil, errors.New("too many products in the order")
	}
	return &order, nil
}
