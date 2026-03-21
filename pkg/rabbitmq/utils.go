package rabbitmq

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	io.Closer
	ch *amqp.Channel
}

func NewConnection(url string) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error

	for i := range 10 {
		conn, err = amqp.Dial(url)
		if err == nil {
			log.Printf("successfully connected to RabbitMQ")
			return conn, nil
		}
		log.Printf("RabbitMQ connection attempt %d/10 failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to RabbitMQ after 10 attempts: %w", err)
}

func NewPublisher(conn *amqp.Connection) (*Publisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error creating publisher: %w", err)
	}

	return &Publisher{
		ch: ch,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, payload []byte) error {
	return p.ch.PublishWithContext(
		ctx,
		exchange,   // exchange name
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			Headers: nil,
			// ContentType — для consumer'а: как десериализовать body.
			// Не влияет на маршрутизацию, чисто информационный header.
			ContentType:     "application/json",
			ContentEncoding: "",
			// DeliveryMode = 2 — persistent. RabbitMQ сохраняет сообщение
			// на диск. При перезапуске RabbitMQ сообщение не потеряется.
			// DeliveryMode = 1 — transient, живёт только в памяти.
			// Для финансовых операций — только persistent.
			DeliveryMode:  amqp.Persistent,
			Priority:      0,
			CorrelationId: "",
			ReplyTo:       "",
			Expiration:    "",
			MessageId:     "",
			Timestamp:     time.Now(),
			Type:          "",
			UserId:        "",
			AppId:         "",
			// Body — сериализованный payload. В нашем случае — JSON
			// с transfer_id, from_id, to_id, amount.
			Body: payload,
		},
	)
}

func (p *Publisher) Close() error {
	return p.ch.Close()
}
