CREATE TABLE accounts (
    -- Идентификатор аккаунта. TEXT а не UUID — клиент может использовать
    -- любой формат (acc_alice, UUID, ULID). Не навязываем.
                          id TEXT PRIMARY KEY,

    -- Баланс. NUMERIC(18,2) — 18 знаков до запятой, 2 после.
    -- Соответствует decimal.Decimal в Go (shopspring).
    -- Никогда не float: 0.1 + 0.2 != 0.3 в float.
                          balance NUMERIC(18, 2) NOT NULL DEFAULT 0,

    -- Валюта. В этом проекте всегда RUB, но колонка нужна
    -- чтобы не смешать рубли с долларами при расширении.
                          currency TEXT NOT NULL DEFAULT 'RUB',

                          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                          updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Последний рубеж защиты. Даже если в коде баг и проверка
    -- баланса в handler'е пропустила отрицательное значение —
    -- PostgreSQL откажет UPDATE на уровне constraint.
    -- Это не бизнес-логика, это страховка уровня storage.
                          CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0)
);