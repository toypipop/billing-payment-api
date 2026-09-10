FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/billing-payment-api .

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/billing-payment-api ./billing-payment-api
COPY openapi.yaml ./openapi.yaml

USER app
EXPOSE 8080

ENTRYPOINT ["/app/billing-payment-api"]
