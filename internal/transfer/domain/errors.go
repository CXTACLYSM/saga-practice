package domain

import "errors"

var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrUndefinedStatus = errors.New("undefined status")
var ErrStatusTransformation = errors.New("forbidden status transformation")
