CREATE TABLE transfers (
    -- Idempotency key от клиента. Клиент генерирует UUID перед отправкой.
    -- При retry с тем же id — ON CONFLICT DO NOTHING в handler'е.
                           id TEXT PRIMARY KEY,

    -- Текущий статус state machine. TEXT а не ENUM:
    -- ALTER TYPE ADD VALUE берёт ACCESS EXCLUSIVE lock на все таблицы
    -- с этим типом. Удалить значение из ENUM невозможно без пересоздания.
    -- TEXT + валидация в domain layer — гибче и безопаснее.
                           status TEXT NOT NULL DEFAULT 'pending',

    -- Аккаунт отправителя. REFERENCES — целостность: нельзя создать
    -- перевод от несуществующего аккаунта.
                           from_id TEXT NOT NULL REFERENCES accounts(id),

    -- Аккаунт получателя.
                           to_id TEXT NOT NULL REFERENCES accounts(id),

    -- Сумма перевода. Тот же NUMERIC(18,2) что и в accounts.
                           amount NUMERIC(18, 2) NOT NULL,

    -- Причина отказа. Заполняется при переходе в failure/compensation статус.
    -- NULL для happy path. Помогает при разборе инцидентов.
                           failure_reason TEXT,

                           created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                           updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Время завершения (completed или compensated). NULL пока в процессе.
    -- Отдельно от updated_at: updated_at меняется при каждом переходе,
    -- completed_at — только при финальном.
                           completed_at TIMESTAMPTZ,

    -- Сумма должна быть положительной. Перевод 0 или -100 не имеет смысла.
                           CONSTRAINT transfers_amount_positive CHECK (amount > 0),

    -- Нельзя перевести деньги самому себе.
                           CONSTRAINT transfers_different_accounts CHECK (from_id != to_id)
    );

-- Мониторинг зависших переводов:
-- SELECT * FROM transfers WHERE status NOT IN ('completed', 'compensated', 'debit_failed')
-- AND created_at < NOW() - INTERVAL '1 minute';
CREATE INDEX idx_transfers_status ON transfers(status);

-- Выборка переводов конкретного аккаунта (история операций).
CREATE INDEX idx_transfers_from_id ON transfers(from_id);
CREATE INDEX idx_transfers_to_id ON transfers(to_id);