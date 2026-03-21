package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/dto"
	"github.com/CXTACLYSM/saga-practice/internal/transfer/application/services"
	"github.com/CXTACLYSM/saga-practice/pkg/shared/domain"
	"github.com/CXTACLYSM/saga-practice/pkg/shared/infrastructure/http/responses"
)

type CreateTransferHandler struct {
	transferService *services.TransferService
}

func NewCreateTransferHandler(transferService *services.TransferService) *CreateTransferHandler {
	return &CreateTransferHandler{
		transferService: transferService,
	}
}

func (h *CreateTransferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var createTransferDTO dto.CreateTransferDTO
	_ = json.NewDecoder(r.Body).Decode(&createTransferDTO)

	transfer, err := h.transferService.Create(r.Context(), createTransferDTO)
	if err != nil {
		var appError *domain.ApplicationError
		if errors.As(err, &appError) {
			log.Printf("[internal server error]: %s", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(responses.ErrorResponse{
				Message: appError.Error(),
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(responses.ErrorResponse{
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(transfer)
}
