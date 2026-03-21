package queries

import (
	"context"
	"fmt"

	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/postgres"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/queries"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FindOneTransferHandler struct {
	pool *pgxpool.Pool
}

func NewFindOneTransferHandler(pool *pgxpool.Pool) *FindOneTransferHandler {
	return &FindOneTransferHandler{
		pool: pool,
	}
}

func (h *FindOneTransferHandler) Handle(ctx context.Context, query queries.FindOneTransferQuery) (*domain.Transfer, error) {
	sql := fmt.Sprintf(`
		SELECT
		    id,
		    status,
		    from_id,
		    to_id,
		    amount,
		    created_at,
		    updated_at
		FROM
		    %s
		WHERE
		    id = $1
	`, postgres.TableTransfers)

	var transfer domain.Transfer
	if err := h.pool.QueryRow(
		ctx,
		sql,
		query.Id,
	).Scan(
		&transfer.Id,
		&transfer.Status,
		&transfer.FromId,
		&transfer.ToId,
		&transfer.Amount,
		&transfer.CreatedAt,
		&transfer.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("error executing find one transfer sql query: %w", err)
	}

	return &transfer, nil
}
