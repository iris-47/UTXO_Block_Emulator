package test

import (
	"blockEmulator/chain"
	"blockEmulator/params"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/core/rawdb"
)

func TestUTXO() {
	Pubkey := []string{"000000000001", "00000000002", "00000000003", "00000000004", "00000000005", "00000000006"}
	pcc := &params.ChainConfig{
		ChainID:        0,
		NodeID:         0,
		ShardID:        0,
		Nodes_perShard: uint64(1),
		ShardNums:      4,
		BlockSize:      uint64(params.MaxBlockSize_global),
		BlockInterval:  uint64(params.Block_Interval),
		InjectSpeed:    uint64(params.InjectSpeed),
	}
	fp := "./record/ldb/s0/N0"
	db, err := rawdb.NewLevelDBDatabase(fp, 0, 1, "accountState", false)
	if err != nil {
		log.Panic(err)
	}
	CurChain, err := chain.NewBlockChain(pcc, db)
	if err != nil {
		log.Panic(err)
	}
	record0 := sha256.Sum256([]byte(Pubkey[0]))
	record1 := sha256.Sum256([]byte(Pubkey[1]))
	record2 := sha256.Sum256([]byte(Pubkey[2]))
	record3 := sha256.Sum256([]byte(Pubkey[3]))

	tx, _ := CurChain.NewUTXOTransaction(Pubkey[0], Pubkey[1], big.NewInt(200))
	CurChain.AddTx2UTXOSet(tx)
	fmt.Println(tx)
	tx, _ = CurChain.NewUTXOTransaction(Pubkey[0], Pubkey[1], big.NewInt(200))
	CurChain.AddTx2UTXOSet(tx)
	fmt.Println(tx)
	tx, _ = CurChain.NewUTXOTransaction(Pubkey[0], Pubkey[2], big.NewInt(300))
	CurChain.AddTx2UTXOSet(tx)
	fmt.Println(tx)

	fmt.Println(CurChain.FindSpendableOutputs(record0[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record1[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record2[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record3[:], big.NewInt(500)))

	tx, _ = CurChain.NewUTXOTransaction(Pubkey[1], Pubkey[3], big.NewInt(500))
	CurChain.AddTx2UTXOSet(tx)
	fmt.Println(tx)
	tx, _ = CurChain.NewUTXOTransaction(Pubkey[1], Pubkey[3], big.NewInt(300))
	CurChain.AddTx2UTXOSet(tx)
	fmt.Println(tx)

	fmt.Println(CurChain.FindSpendableOutputs(record0[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record1[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record2[:], big.NewInt(500)))
	fmt.Println(CurChain.FindSpendableOutputs(record3[:], big.NewInt(500)))
}
