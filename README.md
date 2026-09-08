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
| `GET` | `/invoices/:id` | Get invoice by ID |

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
