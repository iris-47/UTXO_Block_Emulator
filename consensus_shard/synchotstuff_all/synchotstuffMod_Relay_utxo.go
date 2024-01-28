package synchotstuff_all

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

type RawUTXORelayExtraHandleMod struct {
	node *SyncHotstuffConsensusNode
	// pointer to pbft data
}

// the diy operation in propose
func (urhm *RawUTXORelayExtraHandleMod) HandleinPropose(pmsg *message.Propose) bool {
	if urhm.node.CurChain.IsValidUTXOBlock(core.DecodeB(pmsg.RequestMsg.Msg.Content)) != nil {
		urhm.node.pl.Nlog.Printf("S%dN%d : not a valid block\n", urhm.node.ShardID, urhm.node.NodeID)
		return false
	}
	urhm.node.requestPool[string(pmsg.Digest)] = pmsg.RequestMsg
	// merge to be a prepare message
	return true
}

// the operation in commit.
func (urhm *RawUTXORelayExtraHandleMod) Commit(pmsg *message.Propose) bool {
	r := urhm.node.requestPool[string(pmsg.Digest)]
	// requestType ...
	block := core.DecodeB(r.Msg.Content)
	urhm.node.pl.Nlog.Printf("S%dN%d : adding the block %d...now height = %d \n", urhm.node.ShardID, urhm.node.NodeID, block.Header.Number, urhm.node.CurChain.CurrentBlock.Header.Number)
	urhm.node.CurChain.AddBlock(block)
	urhm.node.pl.Nlog.Printf("S%dN%d : added the block %d... \n", urhm.node.ShardID, urhm.node.NodeID, block.Header.Number)
	urhm.node.CurChain.PrintBlockChain()

	// now try to relay txs to other shards(for main nodes)
	if urhm.node.NodeID == urhm.node.view {
		urhm.node.pl.Nlog.Printf("S%dN%d : main node is trying to send relay txs at height = %d \n", urhm.node.ShardID, urhm.node.NodeID, block.Header.Number)
		// generate relay pool and collect txs excuted
		txExcuted := make([]*core.UTXOTransaction, 0)
		urhm.node.CurChain.UTXOTxpool.RelayPool = make(map[uint64][]*core.UTXOTransaction)
		relay1Txs := make([]*core.UTXOTransaction, 0)
		for _, tx := range block.UTXO {
			// Coinbase Tx cannot be Relay Tx
			if tx.IsCoinbase() {
				txExcuted = append(txExcuted, tx)
				continue
			}
			rsid := urhm.node.CurChain.Get_PartitionMap(tx.Recipient)
			if rsid != urhm.node.ShardID {
				ntx := tx
				ntx.Relayed = true
				urhm.node.CurChain.UTXOTxpool.AddRelayTx(ntx, rsid)
				relay1Txs = append(relay1Txs, tx)
			} else {
				txExcuted = append(txExcuted, tx)
			}
		}
		// send relay txs
		for sid := uint64(0); sid < urhm.node.ChainConfig.ShardNums; sid++ {
			if sid == urhm.node.ShardID || len(urhm.node.CurChain.Txpool.RelayPool[sid]) == 0 {
				continue
			}
			relay := message.RelayUTXO{
				Txs:           urhm.node.CurChain.UTXOTxpool.RelayPool[sid],
				SenderShardID: urhm.node.ShardID,
				SenderSeq:     urhm.node.sequenceID,
			}
			rByte, err := json.Marshal(relay)
			if err != nil {
				log.Panic()
			}
			msg_send := message.MergeMessage(message.CRelay, rByte)
			go networks.TcpDial(msg_send, urhm.node.ip_nodeTable[sid][0])
			urhm.node.pl.Nlog.Printf("S%dN%d : sended relay txs to %d\n", urhm.node.ShardID, urhm.node.NodeID, sid)
		}
		urhm.node.CurChain.UTXOTxpool.ClearRelayPool()
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
			SenderShardID:   urhm.node.ShardID,
			ProposeTime:     r.ReqTime,
			CommitTime:      time.Now(),
		}
		bByte, err := json.Marshal(bim)
		if err != nil {
			log.Panic()
		}
		msg_send := message.MergeMessage(message.CBlockInfo, bByte)
		go networks.TcpDial(msg_send, urhm.node.ip_nodeTable[params.DeciderShard][0])
		urhm.node.pl.Nlog.Printf("S%dN%d : sended excuted txs\n", urhm.node.ShardID, urhm.node.NodeID)
		urhm.node.CurChain.Txpool.GetLocked()
		urhm.node.writeCSVline([]string{strconv.Itoa(len(urhm.node.CurChain.Txpool.TxQueue)), strconv.Itoa(len(txExcuted)), strconv.Itoa(int(bim.Relay1TxNum))})
		urhm.node.CurChain.Txpool.GetUnlocked()
	}
	return true
}

func (urhm *RawUTXORelayExtraHandleMod) HandleinVote(*message.Vote) bool {
	// didn't need to do anything in this version
	return true
}

func (urhm *RawUTXORelayExtraHandleMod) HandleReqestforOldSeq(*message.RequestOldMessage) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// the operation for sequential requests
func (urhm *RawUTXORelayExtraHandleMod) HandleforSequentialRequest(som *message.SendOldMessage) bool {
	if int(som.SeqEndHeight-som.SeqStartHeight+1) != len(som.OldRequest) {
		urhm.node.pl.Nlog.Printf("S%dN%d : the SendOldMessage message is not enough\n", urhm.node.ShardID, urhm.node.NodeID)
	} else { // add the block into the node pbft blockchain
		for height := som.SeqStartHeight; height <= som.SeqEndHeight; height++ {
			r := som.OldRequest[height-som.SeqStartHeight]
			if r.RequestType == message.BlockRequest {
				b := core.DecodeB(r.Msg.Content)
				urhm.node.CurChain.AddBlock(b)
			}
		}
		urhm.node.sequenceID = som.SeqEndHeight + 1
		// urhm.node.CurChain.PrintBlockChain()
	}
	return true
}
