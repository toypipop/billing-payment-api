# Simple Billing & Payment API

REST API for creating invoices, viewing balances, and allocating unit payments.

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

Run all commands below from the project root. The API entry point is
`main.go` in `billing-payment-api/`.

Run API server:

```bash
go run .
```

For automatic reload on Go file changes, install Air:

```bash
go install github.com/air-verse/air@v1.64.5
```

Then run:

```bash
air
```

The included `.air.toml` builds `.` and watches Go files throughout the
project, including `internal`. Build output is stored in `tmp/`.
If `air` is not found, add your Go bin directory (usually `%USERPROFILE%\go\bin`
on Windows) to `PATH`.

Server URL:

```text
http://localhost:8080
```

If port `8080` is already used:

```powershell
$env:PORT="8081"
go run .
```

When using Air, run `air` instead of `go run .` after setting `PORT`.

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
| `GET` | `/invoices?unit=A101` | Get invoices for one unit, ordered by ID ascending; returns `[]` when none match |
| `GET` | `/invoices/:id` | Get invoice by ID |
| `POST` | `/payments` | Allocate a payment across a unit's outstanding invoices |

`POST /invoices` request:

```json
{
  "unit": "A101",
  "due_date": "2026-08-01",
  "items": [
    { "description": "Common Fee", "amount_thb": 1500 },
    { "description": "Water Fee", "amount_thb": 300 }
  ]
}
```

`unit` is the room number. Existing rooms are reused and their `updated_at`
is refreshed; missing rooms are created. Each successful POST creates a new
invoice with an automatically generated invoice number. Room and invoice writes
run in one transaction.

`due_date` must be a valid `YYYY-MM-DD` date. `amount_thb` is in baht, is required,
and accepts zero or positive values with up to two decimal places. Amounts are
stored internally as integer satang; all request and response money fields use
THB JSON numbers (`amount_thb`, `total_amount_thb`, `paid_amount_thb`, and
`outstanding_amount_thb`). The example totals `1800.00` THB. Requests do not require
`invoice_number` or `unit_id`.

An invoice must have at least one item. Unit numbers and item descriptions are
trimmed and must not be blank. Invalid input returns `400`; an unknown invoice
ID returns `404`.

Use `GET /invoices?unit=A101` to find invoices by unit. The filter is an exact,
case-sensitive match after trimming surrounding whitespace. An absent filter
returns all invoices; an unknown unit returns `200` with `[]`. An empty, repeated,
or longer-than-50-character `unit` filter returns `400` with code `INVALID_UNIT`.

This changes the public money contract: replace the old `amount` and
`*_amount_cents` fields with the THB fields above. Response numbers are in baht,
not satang. Existing database values are preserved without currency conversion.

### Invoice balances and status

Create, get-by-ID, and list responses include `unit`, `paid_amount_thb`,
`outstanding_amount_thb`, and `status`, alongside the existing invoice number,
unit ID, due date, total, timestamps, and items. For the example above, the
balance fields initially are:

```json
{
  "unit": "A101",
  "total_amount_thb": 1800.00,
  "paid_amount_thb": 0,
  "outstanding_amount_thb": 1800.00,
  "status": "UNPAID"
}
```

`outstanding_amount_thb = total_amount_thb - paid_amount_thb`.
Status is calculated from those amounts rather than stored separately:

| Status | Condition |
| --- | --- |
| `UNPAID` | Nothing paid and total is greater than zero |
| `PARTIAL` | Paid amount is greater than zero but below the total |
| `PAID` | Paid amount equals the total; includes zero-total invoices |

Existing invoices receive a paid amount of zero when the schema is upgraded.

### Payment allocation

```http
POST /payments
Content-Type: application/json

{"unit":"A101","amount_thb":1200}
```

`amount_thb` is in baht and must be greater than zero, with at most two decimal
places, fitting in an `int64` number of satang internally. Responses use THB
numbers with two decimal places, for example `599.00` or `0.29`.
The unit must already exist. Payments target a unit; callers cannot select an
individual invoice or bypass the allocation order.

Allocation uses oldest `due_date` first, then `invoice_number` ascending
(case-sensitive byte order) for matching dates. This is not creation-ID order.
All outstanding invoices qualify, including those not yet overdue. Already-paid
and zero-total invoices are skipped.

For INV001 = 1,000 baht due 2026-08-01 and INV002 = 500 baht due 2026-08-15,
a payment of 1,200 baht returns `201` with the following allocation fields
(actual responses also include payment ID, unit ID, and creation time):

```json
{
  "unit": "A101",
  "amount_thb": 1200.00,
  "allocations": [
    {
      "invoice_id": 1,
      "invoice_number": "INV001",
      "amount_thb": 1000.00,
      "paid_amount_thb": 1000.00,
      "outstanding_amount_thb": 0,
      "status": "PAID"
    },
    {
      "invoice_id": 2,
      "invoice_number": "INV002",
      "amount_thb": 200.00,
      "paid_amount_thb": 200.00,
      "outstanding_amount_thb": 300.00,
      "status": "PARTIAL"
    }
  ]
}
```

Invoice numbers above are illustrative; `POST /invoices` generates unique
`INV-...` numbers automatically. Each allocation's `amount_thb` is the portion
of this payment assigned to that invoice; `paid_amount_thb` is the cumulative
paid amount after this payment.

**Overpayment policy:** reject the entire payment with `409` if it exceeds the
unit's outstanding balance. No payment, allocation, or balance change is saved.
This keeps the API from holding an untracked surplus: it does not maintain a
credit wallet or initiate refunds. Submit an amount at or below the remaining
balance. If all invoices are already paid, a new payment also returns `409`.

| Result | HTTP status |
| --- | --- |
| Full payment, partial payment, or one payment across multiple invoices | `201` |
| Missing/blank unit, missing/zero/negative amount, invalid precision, overflow, or malformed JSON | `400` |
| Unit not found | `404` |
| Unit has no outstanding invoices, or payment exceeds its outstanding balance | `409` |
| Database failure; all payment writes are rolled back | `500` |

Payments, allocation records, and invoice balance updates commit in one database
transaction. A lock on the unit serializes simultaneous payments for that unit;
invoice rows are also locked during allocation. Invoice creation uses the same
unit lock via its upsert. Different units can be processed independently.

Each successful POST records a new payment. There is no idempotency key support;
repeating a successful request may record another payment if enough debt remains.
This endpoint records payments; it does not charge a bank account or card.

### Automated checks

`go test ./...` runs unit tests; PostgreSQL integration tests are skipped unless
`TEST_DATABASE_URL` is set. Integration tests cover creation, balances, ordering,
partial/full payments, paid invoices, invalid requests, overpayments, rollback,
and concurrent payments. They use uniquely named temporary schemas; invoice
tests roll back their schema transaction, while payment tests commit isolated
fixtures for concurrency checks and drop their schema during cleanup. Run with
an account allowed to create schemas:

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
5. Run `Get invoices by unit`; change `@unit` at the top to select a unit
6. Run `Create payment`, then get the invoice again to inspect its balance and status

`requests.http` contains five core requests. Change `@unit` to a fresh unit name
for an isolated example: creating one 1,800-baht invoice and paying 599 baht
leaves 1,201 baht outstanding with status `PARTIAL`. Repeating `Create invoice`
creates another invoice; payments always go to the unit's oldest outstanding
invoices first. Additional edge cases are covered by automated tests above.

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
- `payments`
- `payment_allocations`

Startup does not seed any room or invoice data.

## 6. HTTP status and logs

| HTTP status | When this API returns it |
| --- | --- |
| `200 OK` | Health check succeeds, or invoices are retrieved; an empty list is `[]` |
| `201 Created` | An invoice or payment is created, with its data in the response |
| `400 Bad Request` | Invalid JSON, missing/invalid fields, or an invalid invoice ID |
| `404 Not Found` | Invoice, unit, or route does not exist |
| `405 Method Not Allowed` | The path exists but the HTTP method is unsupported; `Allow` lists supported methods |
| `409 Conflict` | Payment exceeds the outstanding balance, or the unit has no unpaid balance |
| `500 Internal Server Error` | Unexpected application/database failure or a recovered panic |
| `503 Service Unavailable` | `/health` cannot reach the database; its ping has a two-second timeout |

`POST /invoices` also returns `Location: /invoices/{id}`. Use the exact routes
above: trailing slashes are not redirected and return `404`.

`204 No Content` is for successful operations without response content; the
current successful endpoints return data, so none uses `204`. `412 Precondition
Failed` applies when a request-header precondition such as `If-Match` fails;
this API does not currently implement conditional requests. Payment balance
conflicts use `409`. See [HTTP semantics, RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html#section-15).

For example, `"amount_thb": 599` is a valid JSON number, while
`"amount_thb": "599"` is a string and returns `400 INVALID_REQUEST`.
A trailing comma in JSON also returns `400`. These are validation/syntax
failures, not `412` precondition failures.

Every request handled by the application receives a server-generated
`X-Request-ID` response header. All error responses share this shape, keeping
the existing `error` message field and adding a stable `code` and `request_id`:

```json
{
  "error": "payment exceeds unit outstanding balance; no payment was recorded",
  "code": "OVERPAYMENT",
  "request_id": "example-request-id"
}
```

Error codes include `INVALID_REQUEST`, `INVALID_INVOICE`, `INVALID_INVOICE_ID`,
`INVALID_PAYMENT`, `INVALID_UNIT`, `INVOICE_NOT_FOUND`, `UNIT_NOT_FOUND`, `ROUTE_NOT_FOUND`,
`METHOD_NOT_ALLOWED`, `OVERPAYMENT`, `NO_OUTSTANDING_INVOICES`, `INTERNAL_ERROR`,
and `DATABASE_UNAVAILABLE`. Database details and panic traces are kept in server
logs; clients receive a public error message and the request ID.

Run `air` or `go run .` to see structured JSON logs in the terminal. Each request
produces one `http_request` entry after processing. It includes timestamp,
level, request ID, method, path (without query parameters), route pattern,
actual HTTP status and status text, latency in milliseconds, and response bytes.
Errors add `error_code` and `error`; internal failures add `causes`, and panics
add `stack`.

| Log level | Responses |
| --- | --- |
| `INFO` | Successful responses and redirects |
| `WARN` | `4xx` responses |
| `ERROR` | `5xx` responses and panics |

Example request log (shown formatted for readability; output is one JSON line):

```json
{
  "time": "2026-09-09T23:00:00+07:00",
  "level": "WARN",
  "msg": "http_request",
  "request_id": "example-request-id",
  "method": "POST",
  "path": "/payments",
  "route": "/payments",
  "status": 409,
  "status_text": "Conflict",
  "latency_ms": 2.5,
  "response_bytes": 153,
  "error_code": "OVERPAYMENT",
  "error": "payment exceeds unit outstanding balance; no payment was recorded"
}
```

Request/response bodies, query strings, authorization headers, and cookies are
not dumped into access logs. GORM's separate SQL output is disabled in the app
to avoid duplicate logs with interpolated values; failures are logged by the
request or startup logger. Startup emits `server_starting`, `startup_failed`,
or `server_failed`. Gin defaults to release mode unless `GIN_MODE` is set.
Air's own output remains separate; `tmp/build-errors.log` is a build log, not
the HTTP access log. Runtime logs go to stdout and are not stored in a file
automatically.

If a panic happens after headers were already sent, the log preserves that
actual status, uses `ERROR`, and sets `response_aborted: true`; the connection
is aborted rather than appending a second response. An abort before headers
uses status `0` in the log because no HTTP response was sent.

Automated logging tests check status/level consistency, error envelopes,
request IDs, empty `204` bodies, `412` logging, internal-error isolation, and
panic recovery. The `204` and `412` cases use middleware test routes; they do
not add endpoints to the application or extra requests to `requests.http`.
