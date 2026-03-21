package queries

import (
	"context"

	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
)

type FindOneTransferQuery struct {
	Id string
}

type FindOneTransferHandler interface {
	Handle(context.Context, FindOneTransferQuery) (*domain.Transfer, error)
}
