package dto

type CreateTransferDTO struct {
	TransferId    string `json:"transfer_id"`
	AccountFromId string `json:"account_from_id"`
	AccountToId   string `json:"account_to_id"`
	Amount        string `json:"amount"`
}
