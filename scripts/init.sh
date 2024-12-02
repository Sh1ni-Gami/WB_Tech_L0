#!/bin/bash
# Создание базы данных
psql -U $POSTGRES_USER -c 'CREATE DATABASE wb_tech;'

# Создание таблиц и добавление данных
psql -v ON_ERROR_STOP=1 -U $POSTGRES_USER -d wb_tech <<-EOSQL
  CREATE TABLE delivery (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    zip TEXT NOT NULL,
    city TEXT NOT NULL,
    address TEXT NOT NULL,
    region TEXT NOT NULL,
    email TEXT NOT NULL
  );

  CREATE TABLE payment (
    transaction TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    currency TEXT NOT NULL,
    provider TEXT NOT NULL,
    amount INTEGER NOT NULL,
    payment_dt INTEGER NOT NULL,
    bank TEXT NOT NULL,
    delivery_cost INTEGER NOT NULL,
    goods_total INTEGER NOT NULL,
    custom_fee INTEGER NOT NULL
  );

  CREATE TABLE items (
    chrt_id INTEGER PRIMARY KEY,
    track_number TEXT NOT NULL,
    price INTEGER NOT NULL,
    rid TEXT NOT NULL,
    name TEXT NOT NULL,
    sale INTEGER NOT NULL,
    size TEXT NOT NULL,
    total_price INTEGER NOT NULL,
    nm_id INTEGER NOT NULL,
    brand TEXT NOT NULL,
    status INTEGER NOT NULL
  );

  CREATE TABLE orders (
    order_uid TEXT PRIMARY KEY,
    track_number TEXT NOT NULL,
    entry TEXT NOT NULL,
    delivery_id INTEGER NOT NULL,
    payment_id TEXT NOT NULL,
    locale TEXT NOT NULL,
    internal_signature TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    delivery_service TEXT NOT NULL,
    shardkey TEXT NOT NULL,
    sm_id INTEGER NOT NULL,
    date_created TIMESTAMP NOT NULL,
    oof_shard TEXT NOT NULL,
    FOREIGN KEY (delivery_id) REFERENCES delivery(id),
    FOREIGN KEY (payment_id) REFERENCES payment(transaction)
  );

  CREATE TABLE order_item_conn (
    order_uid TEXT NOT NULL,
    chrt_id INTEGER NOT NULL,
    PRIMARY KEY (order_uid, chrt_id),
    FOREIGN KEY (order_uid) REFERENCES orders(order_uid),
    FOREIGN KEY (chrt_id) REFERENCES items(chrt_id)
  );
EOSQL
