# saga-practice

Production-ready демонстрация распределённых транзакций на Go: Saga pattern, Outbox, State Machine, Compensating Transactions.

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

`Generic Process[S Status]` — это кодовое выражение этого паттерна. Transfer и Order встраивают `Process[S]` и определяют только свои конкретные статусы и правила переходов.

## State Machine: Transfer

```
pending
  │
  ├─[debit OK]──────→ debit_applied
  │                       │
  ├─[debit FAIL]──→ debit_failed     ├─[credit OK]──→ completed ✅
                                      │
                                      ├─[credit FAIL]─→ credit_failed
                                                            │
                                                      compensating
                                                            │
                                                      compensated ✅
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
| Язык | Go 1.22+ |
| База данных | PostgreSQL 16 |
| Брокер сообщений | RabbitMQ 3.13 |
| HTTP Router | chi/v5 |
| PostgreSQL driver | pgx/v5 |
| RabbitMQ client | amqp091-go |
| Финансовые суммы | shopspring/decimal |
| Миграции | golang-migrate/v4 |
| Метрики | prometheus/client_golang |
| Конфигурация | spf13/viper |
| Тестирование | stretchr/testify |

## Структура проекта

```
saga-practice/
├── cmd/
│   └── payments/
│       └── main.go                         # Точка входа: DI, graceful shutdown
├── internal/
│   └── transfer/                           # Bounded context: финансовые переводы
│       ├── domain/                         # Чистый домен без зависимостей от инфраструктуры
│       │   ├── process.go                  # Generic Process[S Status]
│       │   ├── transfer.go                 # Transfer entity + state machine
│       │   ├── order.go                    # Order entity (демонстрация изоструктурности)
│       │   ├── account.go                  # Account value object
│       │   ├── errors.go                   # Доменные ошибки
│       │   └── events.go                   # Доменные события
│       ├── application/                    # Use-case координация
│       │   ├── commands/
│       │   │   └── create_transfer.go      # Accept: FOR UPDATE → transfer + outbox → commit
│       │   ├── queries/
│       │   │   ├── get_transfer.go
│       │   │   └── list_transfers.go
│       │   └── ports.go                    # Интерфейсы репозиториев
│       ├── infrastructure/                 # Реализации портов
│       │   ├── postgres/
│       │   │   ├── transfer_repo.go
│       │   │   ├── account_repo.go
│       │   │   └── outbox_repo.go
│       │   ├── rabbitmq/
│       │   │   ├── publisher.go
│       │   │   ├── debit_consumer.go
│       │   │   ├── credit_consumer.go
│       │   │   └── compensate_consumer.go
│       │   ├── worker/
│       │   │   └── outbox_worker.go        # SKIP LOCKED polling
│       │   └── http/
│       │       ├── router.go
│       │       └── transfer_handler.go
│       └── di/
│           └── container.go                # Сборка зависимостей
├── configs/
│   └── config.go                           # Конфигурация через viper
├── pkg/
│   ├── postgres/
│   │   └── pool.go                         # pgxpool.Pool с конфигом
│   └── rabbitmq/
│       └── connection.go                   # Подключение с retry
├── builds/
│   └── local/
│       ├── docker-compose.yml              # PostgreSQL + RabbitMQ + Prometheus + Grafana
│       └── rabbitmq/
│           └── definitions.json            # Exchanges, queues, bindings, DLX
├── migrations/
│   ├── 001_create_accounts.up.sql
│   ├── 001_create_accounts.down.sql
│   ├── 002_create_transfers.up.sql
│   ├── 002_create_transfers.down.sql
│   ├── 003_create_outbox.up.sql
│   └── 003_create_outbox.down.sql
├── scripts/
│   ├── seed.sql                            # Тестовые аккаунты
│   └── check_invariants.sh                 # Проверка инвариантов
├── Makefile
├── go.mod
└── go.sum
```

Зависимости идут только внутрь: `infrastructure → application → domain`. Domain не импортирует ничего из infrastructure.

## Архитектурные решения

**DDD** — домен содержит сложные инварианты (state machine, компенсации, идемпотентность), которые должны быть тестируемы без поднятия PostgreSQL или RabbitMQ.

**CQRS** — Accept (создание перевода) меняет состояние внутри транзакции с FOR UPDATE. GetTransfer/ListTransfers — только чтение, без блокировок. Разделение на уровне кода, не на уровне БД.

**Outbox** — решает dual-write problem. INSERT в outbox — часть той же транзакции, что и INSERT в transfers. Outbox worker отдельно публикует в RabbitMQ.

**SKIP LOCKED** — параллельные outbox worker'ы берут разные записи без contention.

**Prefetch=1** — в буфере consumer'а не более одного неподтверждённого сообщения. При падении теряется максимум одна операция, и идемпотентность consumer'а обеспечивает корректность.

**TEXT для status** — PostgreSQL ENUM требует `ALTER TYPE` миграций с тяжёлыми lock'ами. TEXT + валидация в domain layer — гибче.

**BIGSERIAL для outbox.sequence_id** — строго монотонный порядок. `created_at` может совпасть при высокой нагрузке.

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
make up          # Docker Compose: PostgreSQL + RabbitMQ + Prometheus + Grafana
make migrate     # Применить миграции
make seed        # Загрузить тестовые аккаунты
make run         # Запустить payments service
make transfer    # Создать тестовый перевод
make check       # Проверить инварианты
```

## Модули

Проект реализуется последовательно, каждый модуль вводит один концепт:

| # | Модуль | Концепт |
|---|---|---|
| 001 | Инфраструктура | Docker Compose + миграции + подключение к PostgreSQL и RabbitMQ |
| 002 | Domain | Transfer entity + state machine + Generic Process[S] |
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