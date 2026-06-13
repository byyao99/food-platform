# Food Platform API

A REST API built with Go + Gin, providing **Menu** and **Order** management. Data is persisted in SQLite via GORM (pure-Go driver, no cgo). The DB file defaults to `food-platform.db` and can be changed with the `DB_PATH` env var.

## Project structure

```
food-platform/
├── main.go                      # entry point, starts the HTTP server
├── go.mod
├── internal/
│   ├── models/models.go         # data models (MenuItem, Order, OrderItem)
│   ├── store/store.go           # GORM + SQLite persistence layer
│   ├── handlers/
│   │   ├── menu.go              # menu handler
│   │   └── order.go             # order handler
│   └── router/router.go         # routes + CORS
└── frontend/                    # Vue 3 + TypeScript SPA (see frontend/README.md)
```

## Running the backend

```bash
go run .                                   # port 8080, DB file ./food-platform.db
PORT=9000 go run .                         # custom port
DB_PATH=/tmp/food.db go run .              # custom SQLite file location
```

The SQLite file and its schema are created automatically on first run (GORM `AutoMigrate`).

## API endpoints

Base URL: `http://localhost:8080/api/v1`

### Menu

| Method | Path          | Description       |
|--------|---------------|-------------------|
| GET    | `/menu`       | List all items    |
| POST   | `/menu`       | Create an item    |
| GET    | `/menu/:id`   | Get a single item |
| PUT    | `/menu/:id`   | Update an item    |
| DELETE | `/menu/:id`   | Delete an item    |

### Orders

| Method | Path           | Description                      |
|--------|----------------|----------------------------------|
| GET    | `/orders`      | List all orders (newest first)   |
| POST   | `/orders`      | Create an order                  |
| GET    | `/orders/:id`  | Get a single order               |
| PUT    | `/orders/:id`  | Update order status and note     |
| DELETE | `/orders/:id`  | Delete an order                  |

Also: `GET /health` for a health check.

## Examples

Create a menu item (`price` is in **integer cents** — 18000 == $180.00):

```bash
curl -X POST localhost:8080/api/v1/menu \
  -H 'Content-Type: application/json' \
  -d '{"name":"Beef Noodles","description":"House braised","price":18000,"category":"Main"}'
```

List endpoints support pagination and sorting via query params, and the response includes a `pagination` block:

- `?limit=` — page size (default 20, max 100)
- `?offset=` — rows to skip (default 0)
- `?sort=` — column to sort by; menu: `name`/`price`/`category`/`available`/`created_at`/`updated_at`; orders: `customer_name`/`total_amount`/`status`/`created_at`/`updated_at`. Unknown columns fall back to the default.
- `?order=` — `asc` or `desc` (menu defaults to `created_at asc`, orders to `created_at desc`)

```bash
curl 'localhost:8080/api/v1/orders?limit=20&offset=0&sort=total_amount&order=desc'
# { "data": [...], "pagination": { "limit": 20, "offset": 0, "total": 42 } }
```

Create an order (`total_amount` is computed by the server from menu prices):

```bash
curl -X POST localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_name":"Alice","items":[{"menu_item_id":"<MENU_ID>","quantity":2}],"note":"No cilantro"}'
```

Update order status:

```bash
curl -X PUT localhost:8080/api/v1/orders/<ORDER_ID> \
  -H 'Content-Type: application/json' \
  -d '{"status":"preparing","note":""}'
```

Order status flow: `pending` → `preparing` → `ready` → `completed`. An order can be `cancelled` from any non-terminal state. `completed` and `cancelled` are terminal. Illegal transitions (e.g. `completed` → `pending`) are rejected with 400.

## Design notes

- **Order snapshots**: each order line records the item name and unit price at order time, so later menu price changes do not affect historical orders.
- **Server-side pricing**: amounts (subtotal, total) are always computed by the server; prices sent by the client are not trusted.
- **Validation**: creating an order checks that each menu item exists and is available; request bodies are validated via Gin binding.
- **Response shape**: successful payloads are wrapped in `{"data": ...}`, errors in `{"error": "..."}`.
- **CORS**: a permissive CORS middleware is enabled for local frontend development.
- **Persistence**: data lives in SQLite (`order_items` is a separate table linked to `orders` with cascade delete). Schema migrations run automatically on startup. To switch to PostgreSQL/MySQL later, swap the GORM driver in `internal/store/store.go` — the rest of the code is unchanged.
- **Money as cents**: all monetary fields are integer cents (`int64`), never floats, to avoid rounding errors. The frontend converts dollars ↔ cents at the edge.
- **Status state machine**: order status changes are validated against an allowed-transition table (`models.OrderStatus.CanTransitionTo`); terminal states cannot be left.
- **Pagination**: list endpoints take `limit`/`offset` and return a `pagination` block (`limit`, `offset`, `total`).
- **Graceful shutdown**: the server traps SIGINT/SIGTERM, drains in-flight requests (10s timeout) via `http.Server.Shutdown`, then closes the DB.
