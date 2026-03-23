package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/postgres"
	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/rabbitmq"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/commands"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type CreateTransferCommandHandler struct {
	pool *pgxpool.Pool
}

func NewCreateTransferCommandHandler(pool *pgxpool.Pool) *CreateTransferCommandHandler {
	return &CreateTransferCommandHandler{
		pool: pool,
	}
}

func (h *CreateTransferCommandHandler) Handle(ctx context.Context, command commands.CreateTransferCommand) (*domain.Transfer, error) {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	defer tx.Rollback(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction error: %w", err)
	}

	var fromBalance decimal.Decimal
	sql := fmt.Sprintf(`
		SELECT
			balance
		FROM 
		    %s
		WHERE id = $1
		FOR UPDATE
	`, postgres.TableAccounts)
	row := tx.QueryRow(
		ctx,
		sql,
		command.AccountFromId,
	)
	if err = row.Scan(&fromBalance); err != nil {
		return nil, fmt.Errorf("error scanning from balance row: %w", err)
	}
	if fromBalance.LessThan(command.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	transfer := domain.NewTransfer(command.TransferId, command.AccountFromId, command.AccountToId, command.Amount)
	sql = fmt.Sprintf(`
		INSERT INTO %s
			(id, status, from_id, to_id, amount, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, postgres.TableTransfers)

	_, err = tx.Exec(
		ctx,
		sql,
		transfer.Id,
		transfer.Status,
		transfer.FromId,
		transfer.ToId,
		transfer.Amount,
		transfer.CreatedAt,
		transfer.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sql execution error: %w", err)
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("error marshalling command to outbox payload: %w", err)
	}
	sql = fmt.Sprintf(`
		INSERT INTO %s
			(id, exchange, routing_key, payload, created_at)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, postgres.TableOutbox)
	_, err = tx.Exec(
		ctx,
		sql,
		command.TransferId,
		rabbitmq.ExchangePayments,
		rabbitmq.PaymentsDebit,
		payload,
		time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("error interting to outbox table: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("error commiting create transfer transaction: %w", err)
	}

	log.Printf("[saga transaction start: transfer_%s]", transfer.Id)

	return &transfer, nil
}
