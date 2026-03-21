# saga-practice

Production-ready distributed transactions in Go — Saga, Outbox, State Machine, Compensating Transactions, RabbitMQ, PostgreSQL.

Финансовые переводы между аккаунтами как vehicle для отработки паттернов, критичных в highload backend-системах.

## Что демонстрирует проект

| Паттерн | Где применяется |
|---|---|
| Row-level lock (`FOR UPDATE`) | Изоляция при проверке баланса отправителя |
| Database transaction (`BEGIN/COMMIT`) | Атомарная фиксация transfer + outbox |
| Outbox pattern | Решение dual-write problem без 2PC |
| `SKIP LOCKED` | Параллельные outbox worker'ы без contention |
| State machine | Transfer как processable entity с явными переходами |
| Saga pattern | Многошаговый распределённый процесс: debit → credit → complete |
| Compensating transaction | Возврат средств при partial failure (debit прошёл, credit упал) |
| At-least-once + idempotency | Manual ACK + проверка статуса = exactly-once business effect |
| Prefetch = 1 | Backpressure в финансовых consumer'ах |
| Dead Letter Queue | Финальная обработка сообщений, которые невозможно обработать |
| Graceful shutdown | Context propagation + SIGTERM, ни одно сообщение не теряется |
| Observability | Prometheus-метрики по переходам состояний, structured logging |

## Архитектурное прозрение

Transfer, Order и любой другой бизнес-процесс — изоструктурны:

```
Любой бизнес-процесс =
    happy path статусы (прошедшее время, факты)
  + failure статусы (один на каждый шаг, где может упасть)
  + compensation статусы (откат уже применённых шагов)
  + terminal статусы (completed / cancelled — выхода нет)
```

## State Machine: Transfer

```
┌─────────┐     ┌───────────────┐     ┌─────────────┐
│ pending │────▶│ debit_applied │────▶│  completed  │ ✅
└─────────┘     └───────────────┘     └─────────────┘
     │                  │
     │ debit            │ credit
     │ failed           │ failed
     ▼   ✅             ▼
┌──────────────┐  ┌───────────────┐
│ debit_failed │  │ credit_failed │
└──────────────┘  └───────────────┘
                         │
                         │ compensate
                         ▼
                   ┌──────────────┐
                   │ compensated  │ ✅
                   └──────────────┘
```

Terminal states: `completed`, `compensated`, `debit_failed`. Из них нет выхода.

## Поток данных

```
HTTP Handler
  └── FOR UPDATE (row lock)
  └── BEGIN / COMMIT (atomicity)
  └── ON CONFLICT (idempotency)
  └── Outbox (dual-write solution)
        │
        ▼
Outbox Worker
  └── SKIP LOCKED (parallel workers)
        │
        ▼
RabbitMQ Exchange (routing)
  └── prefetch=1 (backpressure)
  └── manual ACK (at-least-once)
  └── Nack/Reject (retry vs DLQ)
        │
        ▼
DebitConsumer → CreditConsumer → CompensateConsumer
  └── idempotent check (state machine read)
  └── state transition via outbox
```

## Стек

| Компонент | Технология |
|---|---|
| Язык | Go 1.24 |
| База данных | PostgreSQL 16 |
| Брокер сообщений | RabbitMQ 3.13 |
| HTTP Router | chi/v5 |
| PostgreSQL driver | pgx/v5 |
| RabbitMQ client | amqp091-go |
| Финансовые суммы | shopspring/decimal |
| Миграции | golang-migrate |
| Метрики | prometheus/client_golang |
| Конфигурация | spf13/viper |
| Тестирование | stretchr/testify |

## Структура проекта

```
saga-practice/
├── cmd/
│   ├── transfers/
│   │   └── main.go                              # Точка входа: DI, graceful shutdown
│   └── rabbitmq-migrate/
│       └── main.go                              # CLI: миграция RabbitMQ-топологии
├── internal/
│   ├── container.go                             # DI container: сборка зависимостей
│   ├── account/
│   │   └── domain/
│   │       └── account.go                       # Account entity
│   ├── outbox/
│   │   ├── domain/
│   │   │   └── outbox.go                        # Outbox entity
│   │   └── infrastructure/
│   │       └── rabbitmq/
│   │           └── worker.go                    # Polling loop: SKIP LOCKED → PUBLISH → DELETE
│   ├── shared/
│   │   └── infrastructure/
│   │       ├── postgres/
│   │       │   └── tables.go                    # Константы имён таблиц
│   │       └── rabbitmq/
│   │           ├── exchanges.go                 # Константы имён exchanges
│   │           └── routings.go                  # Константы routing keys
│   └── transfer/
│       ├── domain/
│       │   ├── transfer.go                      # Transfer entity + state machine + transitions
│       │   └── errors.go                        # Доменные ошибки
│       ├── application/
│       │   ├── commands/
│       │   │   └── create.go                    # CreateTransferCommand + handler interface
│       │   ├── queries/
│       │   │   └── find_one.go                  # FindOneTransferQuery + handler interface
│       │   ├── dto/
│       │   │   └── create.go                    # CreateTransferDTO (JSON boundary)
│       │   └── services/
│       │       └── transfer.go                  # TransferService: координация commands/queries
│       └── infrastructure/
│           ├── http/
│           │   ├── router.go                    # chi router, middleware
│           │   └── handlers/
│           │       └── create.go                # POST /transfers → service → response
│           ├── postgres/
│           │   ├── commands/
│           │   │   └── create_transfer.go       # FOR UPDATE → INSERT transfer → INSERT outbox → COMMIT
│           │   └── queries/
│           │       └── find_one.go              # SELECT transfer by ID
│           └── rabbitmq/
│               ├── debit.go                     # Debit consumer (TODO)
│               └── credit.go                    # Credit consumer (TODO)
├── configs/
│   ├── config.go                                # Root config: собирает все секции
│   ├── app/
│   │   └── config.go                            # HTTP server: port, timeouts
│   ├── database/
│   │   └── persistence/
│   │       └── postgres/
│   │           └── config.go                    # PostgreSQL: host, port, credentials
│   └── messaging/
│       └── rabbitmq/
│           └── config.go                        # RabbitMQ: host, port, credentials
├── pkg/
│   ├── postgres/
│   │   └── connector.go                         # pgxpool.Pool с конфигом и Ping
│   ├── rabbitmq/
│   │   └── utils.go                             # NewConnection с retry, Publisher
│   └── shared/
│       ├── domain/
│       │   └── errors.go                        # Общие ошибки
│       └── infrastructure/
│           └── http/
│               ├── responses/
│               │   └── standard.go              # Стандартные HTTP-ответы
│               └── utils.go                     # HTTP утилиты
├── builds/
│   ├── Dockerfile                               # Multi-stage build Go-сервиса
│   ├── rabbitmq-migrate.Dockerfile              # Образ для RabbitMQ-миграции
│   └── local/
│       ├── grafana/
│       │   └── provisioning/
│       │       ├── dashboards/
│       │       │   └── dashboard-provider.yml
│       │       └── datasources/
│       │           └── datasource.yml           # Prometheus как datasource
│       ├── prometheus/
│       │   └── prometheus.yml                   # Scrape: Go service + RabbitMQ
│       └── rabbitmq/
│           └── enabled_plugins                  # management + prometheus plugins
├── migrations/
│   ├── 000001_create_accounts_table.up.sql      # accounts: balance CHECK >= 0
│   ├── 000001_create_accounts_table.down.sql
│   ├── 000002_create_transfers_table.up.sql     # transfers: state machine, REFERENCES, constraints
│   ├── 000002_create_transfers_table.down.sql
│   ├── 000003_create_outbox_table.up.sql        # outbox: BIGSERIAL sequence_id, JSONB payload
│   └── 000003_create_outbox_table.down.sql
├── scripts/
│   └── seeds/
│       └── accounts.sql                         # Alice 10000, Bob 5000, Charlie 0 = 15000 total
├── compose.yml
├── Makefile
├── go.mod
└── go.sum
```

Зависимости идут только внутрь: `infrastructure → application → domain`. Domain не импортирует ничего из infrastructure.

## Архитектурные решения

**DDD** — домен содержит сложные инварианты (state machine, компенсации, идемпотентность), которые должны быть тестируемы без поднятия PostgreSQL или RabbitMQ.

**CQRS** — Accept (создание перевода) меняет состояние внутри транзакции с FOR UPDATE. GetTransfer — только чтение, без блокировок. Разделение на уровне кода, не на уровне БД.

**Outbox** — решает dual-write problem. INSERT в outbox — часть той же транзакции, что и INSERT в transfers. Outbox worker отдельно публикует в RabbitMQ.

**SKIP LOCKED** — параллельные outbox worker'ы берут разные записи без contention.

**Prefetch=1** — в буфере consumer'а не более одного неподтверждённого сообщения. При падении теряется максимум одна операция, и идемпотентность consumer'а обеспечивает корректность.

**TEXT для status** — PostgreSQL ENUM требует `ALTER TYPE` миграций с тяжёлыми lock'ами. TEXT + валидация в domain layer — гибче.

**BIGSERIAL для outbox.sequence_id** — строго монотонный порядок. `created_at` может совпасть при высокой нагрузке.

**RabbitMQ, а не Kafka** — saga требует per-message ACK, routing по типу шага и нативный DLQ. Kafka решает другую задачу — high-throughput event streaming. Для финансовых state machine RabbitMQ даёт нужные гарантии из коробки, в Kafka их пришлось бы строить руками.

## Схема БД

**accounts** — баланс аккаунта, `CHECK (balance >= 0)` как последний рубеж защиты от отрицательного баланса.

**transfers** — бизнес-сущность, один перевод = одна строка со state machine. `id` — idempotency key от клиента.

**outbox** — dual-write solution. `sequence_id BIGSERIAL` для FIFO-порядка, `routing_key` для маршрутизации в RabbitMQ.

## RabbitMQ-топология

```
Exchange: "payments" (direct, durable)
├── payment.debit      → Queue: payment.debit      (DLX: payments.dlx)
├── payment.credit     → Queue: payment.credit     (DLX: payments.dlx)
└── payment.compensate → Queue: payment.compensate  (DLX: payments.dlx)

Exchange: "payments.dlx" (fanout, durable)
└── Queue: payment.dead-letter
```

Топология управляется через `make rabbit-migrate` — отдельный CLI-инструмент и Docker-образ. Все Declare-операции в AMQP идемпотентны — безопасно запускать повторно.

## Инварианты

Три проверки после каждого изменения:

```bash
# 1. Race detector
go test -race ./...

# 2. Сумма балансов = const (деньги не появляются и не исчезают)
psql -c "SELECT SUM(balance) FROM accounts;"  # = 15000.00

# 3. Нет зависших переводов (все в terminal state или свежие)
psql -c "SELECT COUNT(*) FROM transfers
         WHERE status NOT IN ('completed', 'compensated', 'debit_failed')
         AND created_at < NOW() - INTERVAL '1 minute';"  # = 0
```

## Быстрый старт

```bash
make up              # Docker Compose: PostgreSQL + RabbitMQ + Prometheus + Grafana
make migrate-up      # Применить SQL-миграции
make rabbit-migrate  # Создать exchanges, queues, bindings, DLX
make seed            # Загрузить тестовые аккаунты
make build           # Собрать Docker-образ Go-сервиса
make transfer        # Создать тестовый перевод
```

## Модули

Проект реализуется последовательно, каждый модуль вводит один концепт:

| # | Модуль | Концепт |
|---|---|---|
| 001 | Инфраструктура | Docker Compose + миграции + подключение к PostgreSQL и RabbitMQ |
| 002 | Domain | Transfer entity + state machine |
| 003 | HTTP Handler | Accept endpoint + FOR UPDATE + Outbox (атомарная фиксация) |
| 004 | Outbox Worker | SKIP LOCKED + publish в RabbitMQ |
| 005 | Debit Consumer | Списание + идемпотентность (at-least-once → exactly-once) |
| 006 | Credit Consumer | Зачисление + переход в terminal state `completed` |
| 007 | Compensate Consumer | Компенсирующая транзакция при partial failure |
| 008 | Dead Letter Queue | Обработка сообщений, которые невозможно обработать |
| 009 | Graceful Shutdown | Context propagation + SIGTERM + корректный порядок остановки |
| 010 | Observability | Prometheus-метрики + structured logging (slog) |

## Лицензия

MIT