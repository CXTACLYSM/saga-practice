FROM golang:1.24.11-alpine3.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o transfers ./cmd/transfers/main.go

FROM alpine:3.23 as transfers

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/transfers .

EXPOSE 8080

CMD ["./transfers"]
