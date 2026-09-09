# Simple Billing & Payment API

REST API for creating and getting invoices.

Current stack:

- Go
- Gin
- PostgreSQL
- GORM
- Docker Compose

## 1. Install

Required:

- Go 1.26+
- Docker Desktop
- VS Code REST Client extension, optional

Install Go dependencies:

```bash
go mod tidy
```

Create `.env` file:

```bash
cp .env.example .env
```

## 2. How to Run

Open Docker Desktop first.

Start PostgreSQL:

```bash
docker compose up -d
```

Run API server:

```bash
go run ./cmd/api
```

Server URL:

```text
http://localhost:8080
```

If port `8080` is already used:

```powershell
$env:PORT="8081"; go run ./cmd/api
```

Run check:

```bash
go test ./...
```

Stop PostgreSQL:

```bash
docker compose down
```

## 3. API Character

Request flow:

```text
Request -> Router -> Handler -> Service -> Repository -> PostgreSQL
```

Endpoints:

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/health` | Check API and database |
| `POST` | `/invoices` | Create invoice |
| `GET` | `/invoices` | Get all invoices with items, ordered by ID ascending; returns `[]` when empty |
| `GET` | `/invoices/:id` | Get invoice by ID |

`POST /invoices` request:

```json
{
  "unit": "A101",
  "due_date": "2026-08-01",
  "items": [
    { "description": "Common Fee", "amount": 1500 },
    { "description": "Water Fee", "amount": 300 }
  ]
}
```

`unit` is the room number. Existing rooms are reused and their `updated_at`
is refreshed; missing rooms are created. Each successful POST creates a new
invoice with an automatically generated invoice number. Room and invoice writes
run in one transaction.

`due_date` must be a valid `YYYY-MM-DD` date. `amount` is in baht, is required,
and accepts zero or positive values with up to two decimal places. Amounts are
stored as integer satang; response fields remain `amount_cents` and
`total_amount_cents` (the example totals `180000`). Requests no longer require
`invoice_number` or `unit_id`.

PostgreSQL integration tests use a temporary schema inside a rolled-back
transaction. Run them with an account allowed to create schemas:

```powershell
$env:TEST_DATABASE_URL="postgres://billing_user:billing_password@localhost:5432/billing_payment?sslmode=disable"
go test ./... -count=1
```

## 4. Test API

### VS Code REST Client

1. Install extension: `REST Client`
2. Open `requests.http`
3. Click `Send Request` on `Health check`
4. Click `Send Request` on `Create invoice`
5. Copy returned `id`
6. Update or run `GET /invoices/{id}`

### curl

```bash
curl http://localhost:8080/health
```

```bash
curl http://localhost:8080/invoices/1
```

## 5. Database

PostgreSQL runs from `docker-compose.yml`.

Default connection:

```text
postgres://billing_user:billing_password@localhost:5432/billing_payment?sslmode=disable
```

Tables are auto-created on API startup:

- `units`
- `invoices`
- `invoice_items`

Startup does not seed any room or invoice data.
