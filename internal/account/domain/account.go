package domain

import "github.com/shopspring/decimal"

type Account struct {
	Id      int
	Balance decimal.Decimal
}
