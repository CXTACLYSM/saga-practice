package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type TransferStatus string

const (
	TransferStatusPending      TransferStatus = "pending"
	TransferStatusDebitFailed  TransferStatus = "debit_failed"
	TransferStatusDebitApplied TransferStatus = "debit_applied"
	TransferStatusCreditFailed TransferStatus = "credit_failed"
	TransferStatusCompensated  TransferStatus = "compensated"
	TransferStatusCompleted    TransferStatus = "completed"
)

var statusTransformations = map[TransferStatus][]TransferStatus{
	TransferStatusPending:      {TransferStatusDebitFailed, TransferStatusDebitApplied},
	TransferStatusDebitFailed:  nil,
	TransferStatusDebitApplied: {TransferStatusCreditFailed, TransferStatusCompleted},
	TransferStatusCreditFailed: {TransferStatusCompensated},
	TransferStatusCompensated:  nil,
	TransferStatusCompleted:    nil,
}

type Transfer struct {
	Id     string
	Status TransferStatus
	FromId string
	ToId   string
	Amount decimal.Decimal

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTransfer(id, fromId, toId string, amount decimal.Decimal) Transfer {
	now := time.Now()
	return Transfer{
		Id:        id,
		Status:    TransferStatusPending,
		FromId:    fromId,
		ToId:      toId,
		Amount:    amount,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (e *Transfer) ToStatus(status TransferStatus) error {
	availableStatuses, ok := statusTransformations[e.Status]
	if !ok {
		return ErrUndefinedStatus
	}

	for _, availableStatus := range availableStatuses {
		if status == availableStatus {
			e.Status = status
			e.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrStatusTransformation
}
