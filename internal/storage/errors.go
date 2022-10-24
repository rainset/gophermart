package storage

import "errors"

var (
	ErrorUserAlreadyExists   = errors.New("user already exists")
	ErrorUserCredentials     = errors.New("wrong pair login/password")
	ErrorUserBalanceWithdraw = errors.New("insufficient funds to withdraw")
	ErrorOrderAlreadyExists  = errors.New("order already exists")
	ErrorOrderNotFound       = errors.New("order not found")
)
