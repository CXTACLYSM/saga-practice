package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	// ── Параметры подключения из командной строки ─────────────────────
	// Аналогия: golang-migrate принимает -database "postgres://..."
	// Мы принимаем отдельные флаги, потому что AMQP URL имеет свой формат.
	host := flag.String("host", "localhost", "RabbitMQ host")
	port := flag.Int("port", 5672, "RabbitMQ AMQP port")
	user := flag.String("user", "guest", "RabbitMQ username")
	password := flag.String("password", "guest", "RabbitMQ password")
	flag.Parse()

	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", *user, *password, *host, *port)
	log.Printf("connecting to RabbitMQ at %s:%d...", *host, *port)

	// ── Подключение с retry ──────────────────────────────────────────
	// RabbitMQ может ещё стартовать когда мы запускаем миграцию.
	// Особенно актуально в CI/CD: docker compose up -d && make rabbit-migrate
	var conn *amqp.Connection
	var err error
	for i := range 10 {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("attempt %d/10 failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("failed to connect after 10 attempts: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open channel: %v", err)
	}
	defer ch.Close()

	// ── Миграция топологии ───────────────────────────────────────────
	// Все Declare-операции в AMQP идемпотентны: если exchange/queue
	// уже существует с теми же параметрами — no-op.
	// Если существует с другими параметрами — ошибка (защита от конфликтов).
	// Поэтому эту команду можно запускать многократно безопасно.

	// ── 1. Dead Letter Exchange ──────────────────────────────────────
	// Fanout: все rejected сообщения из всех очередей попадают сюда.
	// Source queue видна в x-death header сообщения — для разбора хватает.
	log.Println("declaring exchange: payments.dlx (fanout)")
	if err := ch.ExchangeDeclare("payments.dlx", "fanout", true, false, false, false, nil); err != nil {
		log.Fatalf("declare payments.dlx: %v", err)
	}

	log.Println("declaring queue: payment.dead-letter")
	if _, err := ch.QueueDeclare("payment.dead-letter", true, false, false, false, nil); err != nil {
		log.Fatalf("declare payment.dead-letter: %v", err)
	}

	if err := ch.QueueBind("payment.dead-letter", "", "payments.dlx", false, nil); err != nil {
		log.Fatalf("bind payment.dead-letter → payments.dlx: %v", err)
	}

	// ── 2. Main Exchange ─────────────────────────────────────────────
	// Direct: routing key определяет в какую очередь попадёт сообщение.
	// "payment.debit" → очередь payment.debit, и так далее.
	log.Println("declaring exchange: payments (direct)")
	if err := ch.ExchangeDeclare("payments", "direct", true, false, false, false, nil); err != nil {
		log.Fatalf("declare payments: %v", err)
	}

	// ── 3. Queues + Bindings ─────────────────────────────────────────
	// Каждая очередь привязана к DLX: при Reject без requeue
	// сообщение уходит в payments.dlx → payment.dead-letter.
	queues := []struct {
		name       string
		routingKey string
	}{
		{name: "payment.debit", routingKey: "payment.debit"},
		{name: "payment.credit", routingKey: "payment.credit"},
		{name: "payment.compensate", routingKey: "payment.compensate"},
	}

	dlxArgs := amqp.Table{
		"x-dead-letter-exchange": "payments.dlx",
	}

	for _, q := range queues {
		log.Printf("declaring queue: %s (DLX → payments.dlx)", q.name)
		if _, err := ch.QueueDeclare(q.name, true, false, false, false, dlxArgs); err != nil {
			log.Fatalf("declare queue %s: %v", q.name, err)
		}

		log.Printf("binding queue: %s → payments exchange (key: %s)", q.name, q.routingKey)
		if err := ch.QueueBind(q.name, q.routingKey, "payments", false, nil); err != nil {
			log.Fatalf("bind queue %s: %v", q.name, err)
		}
	}

	log.Println("RabbitMQ topology migration complete")
}
