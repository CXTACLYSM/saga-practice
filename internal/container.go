package internal

import (
	"fmt"

	"github.com/CXTACLYSM/saga-practice/configs"
	"github.com/CXTACLYSM/saga-practice/internal/outbox/infrastructure/rabbitmq"
	transferCommands "github.com/CXTACLYSM/saga-practice/internal/transfer/application/commands"
	transferQueries "github.com/CXTACLYSM/saga-practice/internal/transfer/application/queries"
	transferServices "github.com/CXTACLYSM/saga-practice/internal/transfer/application/services"
	transferHTTPHandlers "github.com/CXTACLYSM/saga-practice/internal/transfer/infrastructure/http/handlers"
	transferCommandHandlers "github.com/CXTACLYSM/saga-practice/internal/transfer/infrastructure/postgres/commands"
	transferQueryHandlers "github.com/CXTACLYSM/saga-practice/internal/transfer/infrastructure/postgres/queries"
	consumers "github.com/CXTACLYSM/saga-practice/internal/transfer/infrastructure/rabbitmq"
	"github.com/CXTACLYSM/saga-practice/pkg/postgres"
	pkgRabbimq "github.com/CXTACLYSM/saga-practice/pkg/rabbitmq"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Infrastructure struct {
	Pool     *pgxpool.Pool
	AmqpConn *amqp.Connection
}

type Consumers struct {
	DebitConsumer  *consumers.DebitConsumer
	CreditConsumer *consumers.CreditConsumer
}

type Handlers struct {
	CreateHandler *transferHTTPHandlers.CreateTransferHandler
}

type Commands struct {
	CreateTransfer transferCommands.CreateTransferHandler
}

type Queries struct {
	FindOneTransfer transferQueries.FindOneTransferHandler
}

type Services struct {
	TransferService *transferServices.TransferService

	Publsher *pkgRabbimq.Publisher
}

type Workers struct {
	OutboxWorker *rabbitmq.Worker
}

type Container struct {
	Infrastructure *Infrastructure
	Commands       *Commands
	Queries        *Queries
	Services       *Services
	Handlers       *Handlers
	Workers        *Workers
	Consumers      *Consumers
}

func NewContainer(cfg *configs.Config) (*Container, error) {
	container := Container{}
	if err := container.initInfrastructure(cfg); err != nil {
		return nil, fmt.Errorf("init infrastructure: %w", err)
	}
	if err := container.initCommands(); err != nil {
		return nil, fmt.Errorf("init commands: %w", err)
	}
	if err := container.initQueries(); err != nil {
		return nil, fmt.Errorf("init queries: %w", err)
	}
	if err := container.initServices(); err != nil {
		return nil, fmt.Errorf("init services: %w", err)
	}
	if err := container.initHandlers(); err != nil {
		return nil, fmt.Errorf("init handlers: %w", err)
	}
	if err := container.initWorkers(); err != nil {
		return nil, fmt.Errorf("init workers: %w", err)
	}
	if err := container.initConsumers(); err != nil {
		return nil, fmt.Errorf("init consumers: %w", err)
	}

	return &container, nil
}

func (c *Container) initInfrastructure(cfg *configs.Config) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.Db,
	)
	pool, err := postgres.NewPool(dsn)
	if err != nil {
		return fmt.Errorf("postgres connection: %w", err)
	}

	amqpURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/",
		cfg.RabbitMQ.Username, cfg.RabbitMQ.Password, cfg.RabbitMQ.Host, cfg.RabbitMQ.Port,
	)
	amqpConn, err := pkgRabbimq.NewConnection(amqpURL)
	if err != nil {
		pool.Close()
		return fmt.Errorf("rabbitmq connection: %w", err)
	}

	c.Infrastructure = &Infrastructure{
		Pool:     pool,
		AmqpConn: amqpConn,
	}

	return nil
}

func (c *Container) initCommands() error {
	c.Commands = &Commands{
		CreateTransfer: transferCommandHandlers.NewCreateTransferCommandHandler(c.Infrastructure.Pool),
	}

	return nil
}

func (c *Container) initQueries() error {
	c.Queries = &Queries{
		FindOneTransfer: transferQueryHandlers.NewFindOneTransferHandler(c.Infrastructure.Pool),
	}

	return nil
}

func (c *Container) initServices() error {
	publisher, err := pkgRabbimq.NewPublisher(c.Infrastructure.AmqpConn)
	if err != nil {
		return fmt.Errorf("error creating rabbitmq publisher: %w", err)
	}

	c.Services = &Services{
		TransferService: transferServices.NewTransferService(c.Commands.CreateTransfer),
		Publsher:        publisher,
	}

	return nil
}

func (c *Container) initHandlers() error {
	c.Handlers = &Handlers{
		CreateHandler: transferHTTPHandlers.NewCreateTransferHandler(c.Services.TransferService),
	}

	return nil
}

func (c *Container) initWorkers() error {
	c.Workers = &Workers{
		OutboxWorker: rabbitmq.NewWorker(c.Infrastructure.Pool, c.Services.Publsher),
	}

	return nil
}

func (c *Container) initConsumers() error {
	debitConsumer, err := consumers.NewDebitConsumer(c.Infrastructure.Pool, c.Infrastructure.AmqpConn)
	if err != nil {
		return fmt.Errorf("new debit_consumer: %w", err)
	}
	creditConsumer, err := consumers.NewCreditConsumer(c.Infrastructure.Pool, c.Infrastructure.AmqpConn)
	if err != nil {
		return fmt.Errorf("new credit_consumer: %w", err)
	}

	c.Consumers = &Consumers{
		DebitConsumer:  debitConsumer,
		CreditConsumer: creditConsumer,
	}

	return nil
}
