package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strings"
	"time"
)

// TxIn defines a transaction input.
type TxIn struct {
	PrevTxId  []byte
	Index     int    // The index of the Vouts in PrevTx, named as Vout in btcd
	Signature []byte // Also not implemented now.
	PubKey    []byte // currently implement as Address
}

// TXOut represents a transaction output
type TxOut struct {
	Value      *big.Int
	PubKeyHash []byte // currently implement as "AddressHash"
}

// UTXOTransaction represents a Bitcoin transaction
type UTXOTransaction struct {
	TxId []byte
	Vin  []TxIn
	Vout []TxOut

	Nonce int64

	Time      time.Time // the time adding in pool
	TxHash    []byte
	Relayed   bool   // used in transaction relaying
	Recipient string // used in relay to indicate the receiver shard
}

// Encode transaction for storing
func (tx UTXOTransaction) Encode() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

// Decode transaction
func DecodeUTXOTx(data []byte) *UTXOTransaction {
	var tx UTXOTransaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&tx)
	if err != nil {
		log.Panic(err)
	}

	return &tx
}

func EncodeOutputs(data []TxOut) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func DecodeOutputs(data []byte) []TxOut {
	var outputs []TxOut

	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&outputs)
	if err != nil {
		log.Panic(err)
	}

	return outputs
}

// Hash returns the hash of the Transaction
func (tx *UTXOTransaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.TxId = []byte{}

	hash = sha256.Sum256(txCopy.Encode())

	return hash[:]
}

// IsCoinbase checks whether the transaction is coinbase
func (tx UTXOTransaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].PrevTxId) == 0 && tx.Vin[0].Index == -1
}

// NewCoinbaseTX creates a new coinbase transaction
// CoinbaseTx cannot be cross-shard, so the field "Recepient" can be ignored
func NewCoinbaseTX(PubkeyHash []byte, val *big.Int, TxHash []byte) *UTXOTransaction {
	txin := TxIn{[]byte{}, -1, nil, []byte{}}
	txout := TxOut{val, PubkeyHash}
	tx := UTXOTransaction{
		Vin:  []TxIn{txin},
		Vout: []TxOut{txout},
	}
	randomGenerator := rand.New(rand.NewSource(time.Now().UnixNano()))
	tx.Nonce = randomGenerator.Int63()
	tx.TxId = tx.Hash()
	if TxHash != nil {
		tx.TxHash = TxHash //  Its a relay tx
	} else {
		tx.TxHash = tx.Hash()
	}

	return &tx
}

// Use for fmt.Print()
func (tx UTXOTransaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- UTXOTransaction %x:", tx.TxId))

	for i, input := range tx.Vin {

		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:      %x", input.PrevTxId))
		lines = append(lines, fmt.Sprintf("       Index:     %d", input.Index))
		// lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}

	for i, output := range tx.Vout {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Addr:   %x", output.PubKeyHash))
	}

	return strings.Join(lines, "\n")
}
