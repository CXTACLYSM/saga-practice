CREATE TABLE outbox (
    -- Строго монотонный порядок обработки. BIGSERIAL — автоинкремент,
    -- PostgreSQL сам проставляет 1, 2, 3... при INSERT.
    -- Почему не created_at: два INSERT в одном тике таймера получат
    -- одинаковый timestamp. sequence_id — всегда уникальный и возрастающий.
    -- Worker делает ORDER BY sequence_id — гарантия FIFO.
                        sequence_id BIGSERIAL NOT NULL,

    -- Бизнес-ключ. Для debit шага: transfer_id.
    -- Для credit шага: transfer_id:credit.
    -- Для compensate: transfer_id:compensate.
    -- ON CONFLICT (id) DO NOTHING в коде — идемпотентная вставка.
                        id TEXT PRIMARY KEY,

    -- В какой exchange отправить. Хранение в таблице делает worker
    -- универсальным — он не знает бизнес-логику маршрутизации.
                        exchange TEXT NOT NULL,

    -- Routing key определяет в какую очередь попадёт сообщение.
    -- "payment.debit", "payment.credit", "payment.compensate".
                        routing_key TEXT NOT NULL,

    -- Тело сообщения. JSONB а не TEXT: можно фильтровать по полям
    -- при отладке (WHERE payload->>'from_id' = 'acc_alice').
                        payload JSONB NOT NULL,

    -- Для мониторинга: если created_at давний, а запись всё ещё здесь —
    -- worker отстаёт или сломан.
                        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ключевой индекс для outbox worker polling:
-- SELECT ... ORDER BY sequence_id LIMIT $batch FOR UPDATE SKIP LOCKED
CREATE INDEX idx_outbox_sequence ON outbox(sequence_id);