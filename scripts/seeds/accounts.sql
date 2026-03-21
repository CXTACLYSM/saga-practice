-- Тестовые аккаунты. Общая сумма: 15000.00 RUB.
-- Этот инвариант проверяется после каждого модуля:
-- SELECT SUM(balance) FROM accounts; → 15000.00

INSERT INTO accounts (id, balance, currency) VALUES
    ('acc_alice',   10000.00, 'RUB'),
    ('acc_bob',     5000.00,  'RUB'),
    ('acc_charlie', 0.00,     'RUB')
ON CONFLICT (id) DO NOTHING;