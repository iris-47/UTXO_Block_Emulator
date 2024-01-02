package chain

import (
	"blockEmulator/core"
	"bytes"
	"encoding/hex"
	"log"
	"math/big"

	"github.com/boltdb/bolt"
)

const blockBucket = "block"

// BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

// Next returns next block starting from the tip
func (i *BlockchainIterator) Next() *core.Block {
	var block *core.Block

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blockBucket))
		encodedBlock := b.Get(i.currentHash)
		block = core.DecodeB(encodedBlock)

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	i.currentHash = block.Header.ParentBlockHash

	return block
}

// FindUTXO finds all unspent transaction by traversing the blockchain, only invoked by Reindex() to reindex UTXO Set
// This Function takes a lot of time
// Not in use
func (bc *BlockChain) FindUTXO() map[string][]core.TxOut {
	UTXO := make(map[string][]core.TxOut)
	spentTXOs := make(map[string][]int)
	bci := &BlockchainIterator{bc.CurrentBlock.Hash, bc.Storage.DataBase}

	for {
		block := bci.Next()

		for _, tx := range block.UTXO {
			txID := hex.EncodeToString(tx.TxId)

		Outputs:
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx { // The output has spent
							continue Outputs
						}
					}
				}
				UTXO[txID] = append(UTXO[txID], out)
			}

			if !tx.IsCoinbase() {
				for _, in := range tx.Vin {
					PrevTxId := hex.EncodeToString(in.PrevTxId)
					spentTXOs[PrevTxId] = append(spentTXOs[PrevTxId], in.Index)
				}
			}
		}

		if len(block.Header.ParentBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

// Finds and returns unspent outputs to be refered in inputs
// This function use utxo set
func (bc *BlockChain) FindSpendableOutputs(pubkeyHash []byte, amount *big.Int) (*big.Int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	accumulated := big.NewInt(0)
	db := bc.Storage.DataBase

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bc.Storage.UTXOBucket))
		cursor := bucket.Cursor()

		for key, val := cursor.First(); key != nil; key, val = cursor.Next() {
			txID := hex.EncodeToString(key)
			outs := core.DecodeOutputs(val)

			for outIdx, out := range outs {
				if bytes.Equal(pubkeyHash, out.PubKeyHash) && accumulated.Cmp(amount) == -1 {
					accumulated = new(big.Int).Add(accumulated, out.Value)
					unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return accumulated, unspentOutputs
}

// Rebuilds the UTXO set, only invoke once when first building the UTXO Set
// Not in use
func (bc *BlockChain) Reindex() {
	db := bc.Storage.DataBase

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(bc.Storage.UTXOBucket))
		if err != nil && err != bolt.ErrBucketNotFound {
			log.Panic(err)
		}

		_, err = tx.CreateBucket([]byte(bc.Storage.UTXOBucket))
		if err != nil {
			log.Panic(err)
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	UTXO := bc.FindUTXO()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bc.Storage.UTXOBucket))

		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			if err != nil {
				log.Panic(err)
			}

			err = b.Put(key, core.EncodeOutputs(outs))
			if err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// Update the UTXO set with transactions from the Block
func (bc *BlockChain) UpdateUTXOSet(block *core.Block) {
	db := bc.Storage.DataBase

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bc.Storage.UTXOBucket))

		for _, tx := range block.UTXO {
			if !tx.IsCoinbase() {
				for _, vin := range tx.Vin {
					updatedOuts := []core.TxOut{}
					outsBytes := b.Get(vin.PrevTxId)
					outs := core.DecodeOutputs(outsBytes)

					// Remove the output which the input cited
					// Seems has some prblem but its correct definetely
					for outIdx, out := range outs {
						if outIdx != vin.Index {
							updatedOuts = append(updatedOuts, out)
						}
					}

					if len(updatedOuts) == 0 {
						err := b.Delete(vin.PrevTxId)
						if err != nil {
							log.Panic(err)
						}
					} else {
						err := b.Put(vin.PrevTxId, core.EncodeOutputs(updatedOuts))
						if err != nil {
							log.Panic(err)
						}
					}

				}

				newOutputs := []core.TxOut{}
				newOutputs = append(newOutputs, tx.Vout...)

				err := b.Put(tx.TxId, core.EncodeOutputs(newOutputs))
				if err != nil {
					log.Panic(err)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// This is done temporarily as a makeshift solution.
// Add a Tx to UTXO Set
func (bc *BlockChain) AddTx2UTXOSet(utxotx *core.UTXOTransaction) {
	if utxotx == nil {
		return
	}
	db := bc.Storage.DataBase

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bc.Storage.UTXOBucket))
		if !utxotx.IsCoinbase() {
			for _, vin := range utxotx.Vin {
				updatedOuts := []core.TxOut{}
				outsBytes := b.Get(vin.PrevTxId)
				outs := core.DecodeOutputs(outsBytes)

				// Remove the output which the input cited
				// Seems has some prblem but its correct definetely
				for outIdx, out := range outs {
					if outIdx != vin.Index {
						updatedOuts = append(updatedOuts, out)
					}
				}

				if len(updatedOuts) == 0 {
					err := b.Delete(vin.PrevTxId)
					if err != nil {
						log.Panic(err)
					}
				} else {
					err := b.Put(vin.PrevTxId, core.EncodeOutputs(updatedOuts))
					if err != nil {
						log.Panic(err)
					}
				}
			}
		}
		newOutputs := []core.TxOut{}
		newOutputs = append(newOutputs, utxotx.Vout...)

		err := b.Put(utxotx.TxId, core.EncodeOutputs(newOutputs))
		if err != nil {
			log.Panic(err)
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}
