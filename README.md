# Billing & Payment API

## 1. Setup and Run Instructions

Install and start [Docker Desktop](https://www.docker.com/products/docker-desktop/).

### Run

Create the local environment file:

```bash
cp .env.example .env
```

On PowerShell:

```powershell
Copy-Item .env.example .env
```

Then run this command from the project directory:

```bash
docker compose up --build -d
```

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8081`

Stop the services:

```bash
docker compose down
```

## 2. API Endpoints

After starting the services, open Swagger UI in your browser:

```text
http://localhost:8081
```

Swagger UI lists every endpoint, request body, response, and HTTP status code.
Use **Try it out** to send requests to the local API.

## 3. Database Choice

This project uses **PostgreSQL**.

PostgreSQL was chosen because it is well suited for financial data that requires reliable transactions and strong data consistency. It also handles complex relational structures and JSON data well, and I am familiar with using it.

## 4. Request / Response Examples

See every endpoint, request, response, and HTTP status code in Swagger UI:

```text
http://localhost:8081
```

Example: create an invoice.

```http
POST /invoices
Content-Type: application/json

{
  "unit": "A101",
  "due_date": "2026-10-01",
  "items": [
    {
      "description": "Common fee",
      "amount_thb": 1500
    }
  ]
}
```

The API returns `201 Created` with the created invoice.

## 5. Assumptions and Trade-offs

- Payments require an `Idempotency-Key` to prevent duplicate records during retries.
- Amounts beyond two decimal places are truncated, so clients should send precise payment values.

## 6. Scaling Considerations

Find the slow part first, then improve it.

- **API:** Use pagination and limit invoice items; consider batching payment allocations as volume grows.
- **Backend:** Tune database connection-pool limits and timeouts.
- **Database:** Use query-specific indexes and materialized views for read-heavy
  reports.
