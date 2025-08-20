package erro

import (
	"errors"
)

var (
	ErrCardTypeInvalid	= errors.New("card type invalid")
	ErrTransactioInvalid = errors.New("transaction is in null")
	ErrNotFound 		= errors.New("item not found")
	ErrTypeInvalid 		= errors.New("moviment type invalid")
	ErrHTTPForbiden		= errors.New("forbiden request")
	ErrUnauthorized 	= errors.New("not authorized")
	ErrServer		 	= errors.New("server identified error")
	ErrUnmarshal 		= errors.New("unmarshal json error")
	ErrUpdate			= errors.New("update unsuccessful")
	ErrForceRollback 	= errors.New("force rollback")
	ErrTimeout			= errors.New("timeout: context deadline exceeded.")
)