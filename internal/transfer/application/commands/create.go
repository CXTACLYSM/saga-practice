package commands

import (
	"context"

	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
	"github.com/shopspring/decimal"
)

type CreateTransferCommand struct {
	TransferId    string          `json:"transfer_id"`
	AccountFromId string          `json:"account_from_id"`
	AccountToId   string          `json:"account_to_id"`
	Amount        decimal.Decimal `json:"amount"`
}

type CreateTransferHandler interface {
	Handle(context.Context, CreateTransferCommand) (*domain.Transfer, error)
}
