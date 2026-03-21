package services

import (
	"context"
	"fmt"

	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/commands"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/dto"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/domain"
	"github.com/shopspring/decimal"
)

type TransferService struct {
	CreateTransfer commands.CreateTransferHandler
}

func NewTransferService(createTransfer commands.CreateTransferHandler) *TransferService {
	return &TransferService{
		CreateTransfer: createTransfer,
	}
}

func (s *TransferService) Create(ctx context.Context, dto dto.CreateTransferDTO) (*domain.Transfer, error) {
	amount, err := decimal.NewFromString(dto.Amount)
	if err != nil {
		return nil, fmt.Errorf("error parsing amount to decimal: %w", err)
	}
	command := commands.CreateTransferCommand{
		TransferId:    dto.TransferId,
		AccountFromId: dto.AccountFromId,
		AccountToId:   dto.AccountToId,
		Amount:        amount,
	}

	transfer, err := s.CreateTransfer.Handle(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("error executing create transfer command: %w", err)
	}

	return transfer, nil
}
