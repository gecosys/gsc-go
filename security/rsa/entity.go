package rsa

import "math/big"

type (
	PublicKey struct {
		E *big.Int
		N *big.Int
	}
)
