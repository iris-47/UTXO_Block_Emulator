// addtional module for new consensus
package pbft_all

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

// simple implementation of pbftHandleModule interface ...
// only for block request and use transaction relay
type RawUTXORelayPbftExtraHandleMod struct {
	pbftNode *PbftConsensusNode
	// pointer to pbft data
}

// propose request with different types
func (rphm *RawUTXORelayPbftExtraHandleMod) HandleinPropose() (bool, *message.Request) {
	// new blocks
	block := rphm.pbftNode.CurChain.GenerateBlock()
	r := &message.Request{
		RequestType: message.BlockRequest,
		ReqTime:     time.Now(),
	}
	r.Msg.Content = block.Encode()

	return true, r
}

// the diy operation in preprepare
func (rphm *RawUTXORelayPbftExtraHandleMod) HandleinPrePrepare(ppmsg *message.PrePrepare) bool {
	if rphm.pbftNode.CurChain.IsValidUTXOBlock(core.DecodeB(ppmsg.RequestMsg.Msg.Content)) != nil {
		rphm.pbftNode.pl.Plog.Printf("S%dN%d : not a valid block\n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID)
		return false
	}
	rphm.pbftNode.pl.Plog.Printf("S%dN%d : the pre-prepare message is correct, putting it into the RequestPool. \n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID)
	rphm.pbftNode.requestPool[string(ppmsg.Digest)] = ppmsg.RequestMsg
	// merge to be a prepare message
	return true
}

// the operation in prepare, and in pbft + tx relaying, this function does not need to do any.
func (rphm *RawUTXORelayPbftExtraHandleMod) HandleinPrepare(pmsg *message.Prepare) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// the operation in commit.
func (rphm *RawUTXORelayPbftExtraHandleMod) HandleinCommit(cmsg *message.Commit) bool {
	r := rphm.pbftNode.requestPool[string(cmsg.Digest)]
	// requestType ...
	block := core.DecodeB(r.Msg.Content)
	rphm.pbftNode.pl.Plog.Printf("S%dN%d : adding the block %d...now height = %d \n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID, block.Header.Number, rphm.pbftNode.CurChain.CurrentBlock.Header.Number)
	rphm.pbftNode.CurChain.AddBlock(block)
	rphm.pbftNode.pl.Plog.Printf("S%dN%d : added the block %d... \n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID, block.Header.Number)
	rphm.pbftNode.CurChain.PrintBlockChain()

	// now try to relay txs to other shards(for main nodes)
	if rphm.pbftNode.NodeID == rphm.pbftNode.view {
		rphm.pbftNode.pl.Plog.Printf("S%dN%d : main node is trying to send relay txs at height = %d \n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID, block.Header.Number)
		// generate relay pool and collect txs excuted
		txExcuted := make([]*core.UTXOTransaction, 0)
		rphm.pbftNode.CurChain.UTXOTxpool.RelayPool = make(map[uint64][]*core.UTXOTransaction)
		relay1Txs := make([]*core.UTXOTransaction, 0)
		for _, tx := range block.UTXO {
			// Coinbase Tx cannot be Relay Tx
			if tx.IsCoinbase() {
				txExcuted = append(txExcuted, tx)
				continue
			}
			rsid := rphm.pbftNode.CurChain.Get_PartitionMap(tx.Recipient)
			if rsid != rphm.pbftNode.ShardID {
				ntx := tx
				ntx.Relayed = true
				rphm.pbftNode.CurChain.UTXOTxpool.AddRelayTx(ntx, rsid)
				relay1Txs = append(relay1Txs, tx)
			} else {
				txExcuted = append(txExcuted, tx)
			}
		}
		// send relay txs
		for sid := uint64(0); sid < rphm.pbftNode.pbftChainConfig.ShardNums; sid++ {
			if sid == rphm.pbftNode.ShardID {
				continue
			}
			relay := message.RelayUTXO{
				Txs:           rphm.pbftNode.CurChain.UTXOTxpool.RelayPool[sid],
				SenderShardID: rphm.pbftNode.ShardID,
				SenderSeq:     rphm.pbftNode.sequenceID,
			}
			rByte, err := json.Marshal(relay)
			if err != nil {
				log.Panic()
			}
			msg_send := message.MergeMessage(message.CRelay, rByte)
			go networks.TcpDial(msg_send, rphm.pbftNode.ip_nodeTable[sid][0])
			rphm.pbftNode.pl.Plog.Printf("S%dN%d : sended relay txs to %d\n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID, sid)
		}
		rphm.pbftNode.CurChain.UTXOTxpool.ClearRelayPool()
		// send txs excuted in this block to the listener
		// add more message to measure more metrics
		MakeupTxExcuted := []*core.Transaction{}
		// only tx.Hash and tx.Time is need in test
		for _, tx := range txExcuted {
			MakeupTx := &core.Transaction{
				TxHash: tx.TxHash,
				Time:   tx.Time,
			}
			MakeupTxExcuted = append(MakeupTxExcuted, MakeupTx)
		}
		MakeupTxRelayed := []*core.Transaction{}
		for _, tx := range relay1Txs {
			MakeupTx := &core.Transaction{
				TxHash: tx.TxHash,
				Time:   tx.Time,
			}
			MakeupTxRelayed = append(MakeupTxRelayed, MakeupTx)
		}
		bim := message.BlockInfoMsg{
			BlockBodyLength: len(block.UTXO),
			ExcutedTxs:      MakeupTxExcuted,
			Epoch:           int(block.Header.Number), // use this field as block height
			Relay1Txs:       MakeupTxRelayed,
			Relay1TxNum:     uint64(len(relay1Txs)),
			SenderShardID:   rphm.pbftNode.ShardID,
			ProposeTime:     r.ReqTime,
			CommitTime:      time.Now(),
		}
		bByte, err := json.Marshal(bim)
		if err != nil {
			log.Panic()
		}
		msg_send := message.MergeMessage(message.CBlockInfo, bByte)
		go networks.TcpDial(msg_send, rphm.pbftNode.ip_nodeTable[params.DeciderShard][0])
		rphm.pbftNode.pl.Plog.Printf("S%dN%d : sended excuted txs\n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID)
		rphm.pbftNode.CurChain.Txpool.GetLocked()
		rphm.pbftNode.writeCSVline([]string{strconv.Itoa(len(rphm.pbftNode.CurChain.Txpool.TxQueue)), strconv.Itoa(len(txExcuted)), strconv.Itoa(int(bim.Relay1TxNum))})
		rphm.pbftNode.CurChain.Txpool.GetUnlocked()
	}
	return true
}

func (rphm *RawUTXORelayPbftExtraHandleMod) HandleReqestforOldSeq(*message.RequestOldMessage) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// the operation for sequential requests
func (rphm *RawUTXORelayPbftExtraHandleMod) HandleforSequentialRequest(som *message.SendOldMessage) bool {
	if int(som.SeqEndHeight-som.SeqStartHeight+1) != len(som.OldRequest) {
		rphm.pbftNode.pl.Plog.Printf("S%dN%d : the SendOldMessage message is not enough\n", rphm.pbftNode.ShardID, rphm.pbftNode.NodeID)
	} else { // add the block into the node pbft blockchain
		for height := som.SeqStartHeight; height <= som.SeqEndHeight; height++ {
			r := som.OldRequest[height-som.SeqStartHeight]
			if r.RequestType == message.BlockRequest {
				b := core.DecodeB(r.Msg.Content)
				rphm.pbftNode.CurChain.AddBlock(b)
			}
		}
		rphm.pbftNode.sequenceID = som.SeqEndHeight + 1
		rphm.pbftNode.CurChain.PrintBlockChain()
	}
	return true
}
