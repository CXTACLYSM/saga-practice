package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/postgres"
	"github.com/CXTACLYSM/saga-practice/internal/shared/infrastructure/rabbitmq"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

type CreditPayload struct {
	Amount        decimal.Decimal `json:"amount"`
	TransferId    string          `json:"transfer_id"`
	AccountFromId string          `json:"account_from_id"`
	AccountToId   string          `json:"account_to_id"`
}

type CreditConsumer struct {
	pool *pgxpool.Pool
	ch   *amqp.Channel
	msgs <-chan amqp.Delivery
}

func NewCreditConsumer(pool *pgxpool.Pool, conn *amqp.Connection) (*CreditConsumer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	err = ch.Qos(1, 0, false)
	if err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}

	msgs, err := ch.Consume(
		rabbitmq.PaymentsCredit,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("consume: %w", err)
	}

	return &CreditConsumer{
		pool: pool,
		ch:   ch,
		msgs: msgs,
	}, nil
}

func (c *CreditConsumer) Start(ctx context.Context) {
	go c.process(ctx)
}

func (c *CreditConsumer) process(ctx context.Context) {
	for {
		select {
		case msg, ok := <-c.msgs:
			if !ok {
				return
			}
			resultCode, err := c.execute(ctx, msg)
			switch resultCode {
			case ResultAck:
				msg.Ack(false)
				log.Printf("[saga transaction end (success)]")
			case ResultRetry:
				msg.Nack(false, true)
			case ResultReject:
				msg.Reject(false)
			default:
				log.Printf("[credit consumer]: undefined result code: %d", resultCode)
			}
			if err != nil {
				log.Printf("[credit consumer]: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *CreditConsumer) execute(ctx context.Context, msg amqp.Delivery) (Result, error) {
	// decode payload
	var payload CreditPayload
	err := json.Unmarshal(msg.Body, &payload)
	if err != nil {
		return ResultReject, fmt.Errorf("unmarshal json: %w", err)
	}

	// begin tx
	tx, err := c.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return ResultRetry, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	sql := fmt.Sprintf(`
    	SELECT
			status
    	FROM
			%s
    	WHERE
    	    id = $1
    	FOR UPDATE
	`, postgres.TableTransfers)
	err = tx.QueryRow(ctx, sql, payload.TransferId).Scan(&status)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultReject, fmt.Errorf("transfer %s not found", payload.TransferId)
		}
		return ResultRetry, fmt.Errorf("select transfer: %w", err)
	}

	if status != string(domain.TransferStatusDebitApplied) {
		return ResultAck, nil
	}

	// select account from for update
	var toBalance decimal.Decimal
	sql = fmt.Sprintf(`
		SELECT
			balance
		FROM
			%s
		WHERE
		    id = $1
		FOR UPDATE
	`, postgres.TableAccounts)
	row := tx.QueryRow(
		ctx,
		sql,
		payload.AccountToId,
	)
	err = row.Scan(&toBalance)
	if err != nil {
		return ResultRetry, fmt.Errorf("scan row from %s: %w", postgres.TableAccounts, err)
	}

	balance := toBalance.Add(payload.Amount)

	// update account from
	sql = fmt.Sprintf(`
		UPDATE
			%s
		SET
			balance = $1
		WHERE
		    id = $2
	`, postgres.TableAccounts)
	_, err = tx.Exec(
		ctx,
		sql,
		balance,
		payload.AccountToId,
	)
	if err != nil {
		return ResultRetry, fmt.Errorf("add %s from balance account_%s: %w", payload.Amount, payload.AccountToId, err)
	}

	// update transfer
	sql = fmt.Sprintf(`
		UPDATE
			%s
		SET
		    status = $1,
		    completed_at = $2
		WHERE
		    id = $3
	`, postgres.TableTransfers)
	_, err = tx.Exec(
		ctx,
		sql,
		domain.TransferStatusCompleted,
		time.Now(),
		payload.TransferId,
	)
	if err != nil {
		return ResultRetry, fmt.Errorf("updating transfer_%s: %w", payload.TransferId, err)
	}

	//commit tx
	err = tx.Commit(ctx)
	if err != nil {
		return ResultRetry, fmt.Errorf("commit transaction: %w", err)
	}

	return ResultAck, nil
}
