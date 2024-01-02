// the define and some operation of txpool

package core

import (
	"sync"
	"time"
)

type UTXOTxPool struct {
	TxQueue   []*UTXOTransaction            // UTXOTransaction Queue
	RelayPool map[uint64][]*UTXOTransaction //designed for sharded blockchain, from Monoxide
	lock      sync.Mutex
	// The pending list is ignored
}

func NewUTXOTxPool() *UTXOTxPool {
	return &UTXOTxPool{
		TxQueue:   make([]*UTXOTransaction, 0),
		RelayPool: make(map[uint64][]*UTXOTransaction),
	}
}

// Add a UTXOTransaction to the pool (consider the queue only)
func (txpool *UTXOTxPool) AddTx2Pool(tx *UTXOTransaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	if tx.Time.IsZero() {
		tx.Time = time.Now()
	}
	txpool.TxQueue = append(txpool.TxQueue, tx)
}

// Add a list of UTXOTransactions to the pool
func (txpool *UTXOTxPool) AddTxs2Pool(txs []*UTXOTransaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	for _, tx := range txs {
		if tx.Time.IsZero() {
			tx.Time = time.Now()
		}
		txpool.TxQueue = append(txpool.TxQueue, tx)
	}
}

// add UTXOTransactions into the pool head
func (txpool *UTXOTxPool) AddTxs2Pool_Head(tx []*UTXOTransaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	txpool.TxQueue = append(tx, txpool.TxQueue...)
}

// Pack UTXOTransactions for a proposal
func (txpool *UTXOTxPool) PackTxs(max_txs uint64) []*UTXOTransaction {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	txNum := max_txs
	if uint64(len(txpool.TxQueue)) < txNum {
		txNum = uint64(len(txpool.TxQueue))
	}
	txs_Packed := txpool.TxQueue[:txNum]
	txpool.TxQueue = txpool.TxQueue[txNum:]
	return txs_Packed
}

// Relay UTXOTransactions
func (txpool *UTXOTxPool) AddRelayTx(tx *UTXOTransaction, shardID uint64) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	_, ok := txpool.RelayPool[shardID]
	if !ok {
		txpool.RelayPool[shardID] = make([]*UTXOTransaction, 0)
	}
	txpool.RelayPool[shardID] = append(txpool.RelayPool[shardID], tx)
}

// txpool get locked
func (txpool *UTXOTxPool) GetLocked() {
	txpool.lock.Lock()
}

// txpool get unlocked
func (txpool *UTXOTxPool) GetUnlocked() {
	txpool.lock.Unlock()
}

// get the length of tx queue
func (txpool *UTXOTxPool) GetTxQueueLen() int {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	return len(txpool.TxQueue)
}

// get the length of ClearRelayPool
func (txpool *UTXOTxPool) ClearRelayPool() {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	txpool.RelayPool = nil
}

// abort ! Pack relay UTXOTransactions from relay pool
func (txpool *UTXOTxPool) PackRelayTxs(shardID, minRelaySize, maxRelaySize uint64) ([]*UTXOTransaction, bool) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	if _, ok := txpool.RelayPool[shardID]; !ok {
		return nil, false
	}
	if len(txpool.RelayPool[shardID]) < int(minRelaySize) {
		return nil, false
	}
	txNum := maxRelaySize
	if uint64(len(txpool.RelayPool[shardID])) < txNum {
		txNum = uint64(len(txpool.RelayPool[shardID]))
	}
	relayTxPacked := txpool.RelayPool[shardID][:txNum]
	txpool.RelayPool[shardID] = txpool.RelayPool[shardID][txNum:]
	return relayTxPacked, true
}

// // abort ! Transfer UTXOTransactions when re-sharding
// func (txpool *UTXOTxPool) TransferTxs(addr utils.Address) []*UTXOTransaction {
// 	txpool.lock.Lock()
// 	defer txpool.lock.Unlock()
// 	txTransfered := make([]*UTXOTransaction, 0)
// 	newTxQueue := make([]*UTXOTransaction, 0)
// 	for _, tx := range txpool.TxQueue {
// 		if tx.Sender == addr {
// 			txTransfered = append(txTransfered, tx)
// 		} else {
// 			newTxQueue = append(newTxQueue, tx)
// 		}
// 	}
// 	newRelayPool := make(map[uint64][]*UTXOTransaction)
// 	for shardID, shardPool := range txpool.RelayPool {
// 		for _, tx := range shardPool {
// 			if tx.Sender == addr {
// 				txTransfered = append(txTransfered, tx)
// 			} else {
// 				if _, ok := newRelayPool[shardID]; !ok {
// 					newRelayPool[shardID] = make([]*UTXOTransaction, 0)
// 				}
// 				newRelayPool[shardID] = append(newRelayPool[shardID], tx)
// 			}
// 		}
// 	}
// 	txpool.TxQueue = newTxQueue
// 	txpool.RelayPool = newRelayPool
// 	return txTransfered
// }
