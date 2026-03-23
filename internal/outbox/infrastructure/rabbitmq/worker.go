package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/CXTACLYSM/saga-practice/internal/outbox/domain"
	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/postgres"
	"github.com/CXTACLYSM/saga-practice/pkg/rabbitmq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	pool      *pgxpool.Pool
	publisher *rabbitmq.Publisher
}

func NewWorker(pool *pgxpool.Pool, publisher *rabbitmq.Publisher) *Worker {
	return &Worker{
		pool:      pool,
		publisher: publisher,
	}
}

func (w *Worker) Start(ctx context.Context) {
	go w.process(ctx)
}

func (w *Worker) process(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[outbox error]: recovered panic in worker")
					}
				}()
				if err := w.execute(ctx); err != nil {
					log.Printf("[outbox error]: %v", err)
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) execute(ctx context.Context) error {
	tx, err := w.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("error begin outbox transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var outbox domain.Outbox
	sql := fmt.Sprintf(`
		SELECT
		    sequence_id, id, exchange, routing_key, payload
		FROM
		    %s
		ORDER BY sequence_id
		LIMIT 1
		FOR UPDATE
		SKIP LOCKED
	`, postgres.TableOutbox)
	row := tx.QueryRow(ctx, sql)
	err = row.Scan(
		&outbox.SequenceId,
		&outbox.Id,
		&outbox.Exchange,
		&outbox.RoutingKey,
		&outbox.Payload,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("error scanning row from outbox: %w", err)
	}

	err = w.publisher.Publish(ctx, outbox.Exchange, outbox.RoutingKey, []byte(outbox.Payload))
	if err != nil {
		return fmt.Errorf(
			"error pushing message to rabbitmq (outbox_id=%s, rabbitmq_exchange=%s, rabbitmq_routing_key=%s): %w",
			outbox.Id, outbox.Exchange, outbox.RoutingKey, err,
		)
	}
	log.Printf(
		"[outbox success]: processed outbox message (sequence_id=%s) to queue (rabbitmq_exchange=%s, rabbitmq_routing_key=%s)",
		outbox.SequenceId, outbox.Exchange, outbox.RoutingKey,
	)

	sql = fmt.Sprintf(`
		DELETE FROM %s WHERE sequence_id = $1
	`, postgres.TableOutbox)
	_, err = tx.Exec(
		ctx,
		sql,
		outbox.SequenceId,
	)
	if err != nil {
		return fmt.Errorf("error deleting from outbox: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error commiting transaction: %w", err)
	}

	return nil
}
