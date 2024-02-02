// To make the speed of finding UTXO faster

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"math/big"
)

type UTXOAccountState struct {
	PubkeyHash []byte
	Nonce      uint64
	Balance    *big.Int
	UTXOs      map[string]map[int]*big.Int // Txid->[OutputIndex->value]
}

// Reduce the balance of an account
func (as *UTXOAccountState) Deduct(val *big.Int) bool {
	if as.Balance.Cmp(val) < 0 {
		return false
	}
	as.Balance.Sub(as.Balance, val)
	return true
}

// Increase the balance of an account
func (s *UTXOAccountState) Deposit(value *big.Int) {
	s.Balance.Add(s.Balance, value)
}

// Encode AccountState in order to store in the MPT
func (as *UTXOAccountState) Encode() []byte {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(as)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// Decode UTXOAccountState
func DecodeUAS(b []byte) *UTXOAccountState {
	var as UTXOAccountState

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&as)
	if err != nil {
		log.Panic(err)
	}
	return &as
}

// Hash AccountState for computing the MPT Root
func (as *UTXOAccountState) Hash() []byte {
	h := sha256.Sum256(as.Encode())
	return h[:]
}
