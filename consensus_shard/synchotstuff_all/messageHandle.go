package synchotstuff_all

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/shard"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"
)

// Trannsform Tx to UTXO Tx
func (node *SyncHotstuffConsensusNode) TxTransform() {
	node.pl.Nlog.Printf("S%dN%d : start Tx Transform! \n", node.ShardID, node.NodeID)
	if node.view != node.NodeID {
		return
	}
	for {
		select {
		case <-node.pStop:
			node.pl.Nlog.Printf("S%dN%d stop, stop TxTransform()...\n", node.ShardID, node.NodeID)
			return
		default:
		}
		injectedTxs := node.CurChain.Txpool.PackTxs(200)

		txs := []*core.UTXOTransaction{}
		for _, InjectTx := range injectedTxs {
			tx, coinbase := node.CurChain.NewUTXOTransaction(InjectTx.Sender, InjectTx.Recipient, InjectTx.Value)
			if coinbase != nil { // coinbase
				txs = append(txs, coinbase)
				txs = append(txs, tx)
				node.CurChain.AddTx2UTXOSet(tx)
			} else if tx == nil { // insufficient
				continue
			} else { // normal
				txs = append(txs, tx)
				node.CurChain.AddTx2UTXOSet(tx)
			}
		}

		node.CurChain.UTXOTxpool.AddTxs2Pool(txs)

		if len(injectedTxs) != 0 {
			node.pl.Nlog.Printf("S%dN%d : has transformed %d injected txs msg to %d UTXO Txs\n", node.ShardID, node.NodeID, len(injectedTxs), len(txs))
		}
	}
}

// this func is only invoked by main node
func (node *SyncHotstuffConsensusNode) Propose() {
	if params.UTXO {
		node.pl.Nlog.Printf("S%dN%d working in UTXO mod...\n", node.ShardID, node.NodeID)
	} else {
		node.pl.Nlog.Printf("S%dN%d working in Account mod...\n", node.ShardID, node.NodeID)
	}
	if node.view != node.NodeID {
		return
	}
	for {
		select {
		case <-node.pStop:
			node.pl.Nlog.Printf("S%dN%d stop, stop Propose()...\n", node.ShardID, node.NodeID)
			return
		default:
		}
		// Set MaxTimeLatency = 5ms
		time.Sleep(4 * time.Duration(params.MaxLatency) * time.Millisecond)

		node.sequenceLock.Lock()
		node.pl.Nlog.Printf("S%dN%d get sequenceLock locked, now trying to propose...\n", node.ShardID, node.NodeID)
		// propose
		block := node.CurChain.GenerateBlock()
		r := &message.Request{
			RequestType: message.BlockRequest,
			ReqTime:     time.Now(),
		}
		r.Msg.Content = block.Encode()

		digest := getDigest(r)
		node.requestPool[string(digest)] = r
		node.pl.Nlog.Printf("S%dN%d put the request into the pool ...\n", node.ShardID, node.NodeID)

		pmsg := message.Propose{
			RequestMsg: r,
			Digest:     digest,
			SeqID:      node.sequenceID,
		}
		node.height2Digest[node.sequenceID] = string(digest)
		// marshal and broadcast
		pbyte, err := json.Marshal(pmsg)
		if err != nil {
			log.Panic()
		}
		msg_send := message.MergeMessage(message.CPropose, pbyte)
		networks.Broadcast(node.RunningNode.IPaddr, node.getNeighborNodes(), msg_send)

		// wait to commit the block
		node.requestPool[string(getDigest(pmsg.RequestMsg))] = pmsg.RequestMsg
		node.height2Digest[pmsg.SeqID] = string(getDigest(pmsg.RequestMsg))

		go node.WaitToCommit(&pmsg)

		// // mimic the CvBlock
		// for {
		// 	if node.cntVoteConfirm[string(digest)] == int(node.node_nums) {
		// 		break
		// 	}
		// }
	}
}

func (node *SyncHotstuffConsensusNode) handlePropose(content []byte) {
	// decode the message
	pmsg := new(message.Propose)
	err := json.Unmarshal(content, pmsg)
	if err != nil {
		log.Panic(err)
	}

	// it has already been commited(block from another vote)
	if _, ok := node.requestPool[string(pmsg.Digest)]; ok {
		vote := message.Vote{
			RequestMsg: pmsg.RequestMsg,
			Digest:     pmsg.Digest,
			SeqID:      pmsg.SeqID,
			SenderNode: node.RunningNode,
		}
		voteByte, err := json.Marshal(vote)
		if err != nil {
			log.Panic()
		}
		// broadcast
		msg_send := message.MergeMessage(message.CVote, voteByte)
		networks.Broadcast(node.RunningNode.IPaddr, node.getNeighborNodes(), msg_send)
		node.pl.Nlog.Printf("S%dN%d : has broadcast the vote message \n", node.ShardID, node.NodeID)
	} else {
		flag := false
		if digest := getDigest(pmsg.RequestMsg); string(digest) != string(pmsg.Digest) {
			node.pl.Nlog.Printf("S%dN%d : the digest is not consistent, so refuse to commit. \n", node.ShardID, node.NodeID)
		} else {
			// do your operation in this interface
			flag = node.ihm.HandleinPropose(pmsg)
			node.requestPool[string(getDigest(pmsg.RequestMsg))] = pmsg.RequestMsg
			node.height2Digest[pmsg.SeqID] = string(getDigest(pmsg.RequestMsg))
		}
		// if the message is true, broadcast the prepare message
		if flag {
			vote := message.Vote{
				RequestMsg: pmsg.RequestMsg,
				Digest:     pmsg.Digest,
				SeqID:      pmsg.SeqID,
				SenderNode: node.RunningNode,
			}
			voteByte, err := json.Marshal(vote)
			if err != nil {
				log.Panic()
			}
			// broadcast
			msg_send := message.MergeMessage(message.CVote, voteByte)
			networks.Broadcast(node.RunningNode.IPaddr, node.getNeighborNodes(), msg_send)
			node.pl.Nlog.Printf("S%dN%d : has broadcast the vote message \n", node.ShardID, node.NodeID)

			// commit
			go node.WaitToCommit(pmsg)
		}
	}
}

func (node *SyncHotstuffConsensusNode) WaitToCommit(pmsg *message.Propose) {
	timer := time.NewTimer(time.Duration(2*params.MaxLatency) * time.Millisecond)
	select {
	case <-node.pBreak:
		timer.Stop()
		node.pl.Nlog.Printf("S%dN%d : Get interrupt, stop commit. \n", node.ShardID, node.NodeID)
		return

	case <-timer.C:
		node.pl.Nlog.Printf("S%dN%d : Time wait enough, now commit. \n", node.ShardID, node.NodeID)
		node.lock.Lock()
		defer node.lock.Unlock()
		// if this node is left behind, so it need to requst blocks
		if _, ok := node.requestPool[string(pmsg.Digest)]; !ok {
			node.isReply[string(pmsg.Digest)] = true
			node.askForLock.Lock()
			// request the block
			sn := &shard.Node{
				NodeID:  node.view,
				ShardID: node.ShardID,
				IPaddr:  node.ip_nodeTable[node.ShardID][node.view],
			}
			orequest := message.RequestOldMessage{
				SeqStartHeight: node.sequenceID + 1,
				SeqEndHeight:   pmsg.SeqID,
				ServerNode:     sn,
				SenderNode:     node.RunningNode,
			}
			bromyte, err := json.Marshal(orequest)
			if err != nil {
				log.Panic()
			}

			node.pl.Nlog.Printf("S%dN%d : is now requesting message (seq %d to %d) ... \n", node.ShardID, node.NodeID, orequest.SeqStartHeight, orequest.SeqEndHeight)
			msg_send := message.MergeMessage(message.CRequestOldrequest, bromyte)
			networks.TcpDial(msg_send, orequest.ServerNode.IPaddr)
		} else {
			// implement interface
			node.ihm.Commit(pmsg)
			node.isReply[string(pmsg.Digest)] = true
			node.pl.Nlog.Printf("S%dN%d: this round of synchotstuff %d is end \n", node.ShardID, node.NodeID, node.sequenceID)
			node.sequenceID += 1
		}

		// if this node is a main node, then unlock the sequencelock
		if node.NodeID == node.view {
			node.sequenceLock.Unlock()
			node.pl.Nlog.Printf("S%dN%d get sequenceLock unlocked...\n", node.ShardID, node.NodeID)
		}
	}
}

func (node *SyncHotstuffConsensusNode) handleVote(content []byte) {
	// decode the message
	vmsg := new(message.Vote)
	err := json.Unmarshal(content, vmsg)
	if err != nil {
		log.Panic(err)
	}

	if node.NodeID == node.view {
		if _, ok := node.cntVoteConfirm[string(vmsg.Digest)]; !ok {
			node.cntVoteConfirm[string(vmsg.Digest)] = 0
		}
		node.cntVoteConfirm[string(vmsg.Digest)] += 1
	}

	// an equivocate vote has received
	if string(vmsg.Digest) != node.height2Digest[vmsg.SeqID] && vmsg.SeqID == node.sequenceID {
		node.pBreak <- 1
		node.pl.Nlog.Printf("S%dN%d get an equivocate vote from node %v...\n", node.ShardID, node.NodeID, vmsg.SenderNode)
	} else if vmsg.SeqID > node.sequenceID {
		if _, ok := node.requestPool[string(vmsg.Digest)]; ok {
			return // a block which has already been commited
		}
		// get a propose from vote message instead of a propose message
		node.pl.Nlog.Printf("S%dN%d get an new vote from node %v...\n", node.ShardID, node.NodeID, vmsg.SenderNode)
		pmsg := &message.Propose{
			RequestMsg: vmsg.RequestMsg,
			Digest:     vmsg.Digest,
			SeqID:      vmsg.SeqID,
		}
		flag := false
		if digest := getDigest(vmsg.RequestMsg); string(digest) != string(pmsg.Digest) {
			node.pl.Nlog.Printf("S%dN%d : the digest is not consistent, so refuse to commit. \n", node.ShardID, node.NodeID)
		} else {
			// do your operation in this interface
			flag = node.ihm.HandleinPropose(pmsg)
			node.requestPool[string(getDigest(pmsg.RequestMsg))] = pmsg.RequestMsg
			node.height2Digest[pmsg.SeqID] = string(getDigest(pmsg.RequestMsg))
		}
		// if the message is true, broadcast the prepare message
		if flag {
			vote := message.Vote{
				RequestMsg: pmsg.RequestMsg,
				Digest:     pmsg.Digest,
				SeqID:      pmsg.SeqID,
				SenderNode: node.RunningNode,
			}
			voteByte, err := json.Marshal(vote)
			if err != nil {
				log.Panic()
			}
			// broadcast
			msg_send := message.MergeMessage(message.CVote, voteByte)
			networks.Broadcast(node.RunningNode.IPaddr, node.getNeighborNodes(), msg_send)
			node.pl.Nlog.Printf("S%dN%d : has broadcast the vote message \n", node.ShardID, node.NodeID)

			// commit
			go node.WaitToCommit(pmsg)
		}
	}
}

// this func is only invoked by the main node,
// if the request is correct, the main node will send
// block back to the message sender.
// now this function can send both block and partition
func (node *SyncHotstuffConsensusNode) handleRequestOldSeq(content []byte) {
	if node.view != node.NodeID {
		return
	}

	rom := new(message.RequestOldMessage)
	err := json.Unmarshal(content, rom)
	if err != nil {
		log.Panic()
	}
	node.pl.Nlog.Printf("S%dN%d : received the old message requst from ...", node.ShardID, node.NodeID)
	rom.SenderNode.PrintNode()

	oldR := make([]*message.Request, 0)
	for height := rom.SeqStartHeight; height <= rom.SeqEndHeight; height++ {
		if _, ok := node.height2Digest[height]; !ok {
			node.pl.Nlog.Printf("S%dN%d : has no this digest to this height %d\n", node.ShardID, node.NodeID, height)
			break
		}
		if r, ok := node.requestPool[node.height2Digest[height]]; !ok {
			node.pl.Nlog.Printf("S%dN%d : has no this message to this digest %d\n", node.ShardID, node.NodeID, height)
			break
		} else {
			oldR = append(oldR, r)
		}
	}
	node.pl.Nlog.Printf("S%dN%d : has generated the message to be sent\n", node.ShardID, node.NodeID)

	node.ihm.HandleReqestforOldSeq(rom)

	// send the block back
	sb := message.SendOldMessage{
		SeqStartHeight: rom.SeqStartHeight,
		SeqEndHeight:   rom.SeqEndHeight,
		OldRequest:     oldR,
		SenderNode:     node.RunningNode,
	}
	sbByte, err := json.Marshal(sb)
	if err != nil {
		log.Panic()
	}
	msg_send := message.MergeMessage(message.CSendOldrequest, sbByte)
	networks.TcpDial(msg_send, rom.SenderNode.IPaddr)
	node.pl.Nlog.Printf("S%dN%d : send blocks\n", node.ShardID, node.NodeID)
}

// node requst blocks and receive blocks from the main node
func (node *SyncHotstuffConsensusNode) handleSendOldSeq(content []byte) {
	som := new(message.SendOldMessage)
	err := json.Unmarshal(content, som)
	if err != nil {
		log.Panic()
	}
	node.pl.Nlog.Printf("S%dN%d : has received the SendOldMessage message\n", node.ShardID, node.NodeID)

	// implement interface for new consensus
	node.ihm.HandleforSequentialRequest(som)
	beginSeq := som.SeqStartHeight
	for idx, r := range som.OldRequest {
		node.requestPool[string(getDigest(r))] = r
		node.height2Digest[uint64(idx)+beginSeq] = string(getDigest(r))
		node.isReply[string(getDigest(r))] = true
		node.pl.Nlog.Printf("this round of pbft %d is end \n", uint64(idx)+beginSeq)
	}
	node.sequenceID = som.SeqEndHeight + 1
	// if rDigest, ok1 := node.height2Digest[node.sequenceID]; ok1 {
	// 	if r, ok2 := node.requestPool[rDigest]; ok2 {
	// 		ppmsg := &message.PrePrepare{
	// 			RequestMsg: r,
	// 			SeqID:      node.sequenceID,
	// 			Digest:     getDigest(r),
	// 		}
	// 		flag := false
	// 		flag = node.ihm.HandleinPrePrepare(ppmsg)
	// 		if flag {
	// 			pre := message.Prepare{
	// 				Digest:     ppmsg.Digest,
	// 				SeqID:      ppmsg.SeqID,
	// 				SenderNode: node.RunningNode,
	// 			}
	// 			prepareByte, err := json.Marshal(pre)
	// 			if err != nil {
	// 				log.Panic()
	// 			}
	// 			// broadcast
	// 			msg_send := message.MergeMessage(message.CPrepare, prepareByte)
	// 			networks.Broadcast(node.RunningNode.IPaddr, node.getNeighborNodes(), msg_send)
	// 			node.pl.Nlog.Printf("S%dN%d : has broadcast the prepare message \n", node.ShardID, node.NodeID)
	// 		}
	// 	}
	// }

	node.askForLock.Unlock()
}

// get neighbor nodes in a shard
func (node *SyncHotstuffConsensusNode) getNeighborNodes() []string {
	receiverNodes := make([]string, 0)
	for _, ip := range node.ip_nodeTable[node.ShardID] {
		receiverNodes = append(receiverNodes, ip)
	}
	return receiverNodes
}

func (node *SyncHotstuffConsensusNode) writeCSVline(str []string) {
	dirpath := params.DataWrite_path + "pbft_" + strconv.Itoa(int(node.ChainConfig.ShardNums))
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}

	targetPath := dirpath + "/Shard" + strconv.Itoa(int(node.ShardID)) + strconv.Itoa(int(node.ChainConfig.ShardNums)) + ".csv"
	f, err := os.Open(targetPath)
	if err != nil && os.IsNotExist(err) {
		file, er := os.Create(targetPath)
		if er != nil {
			panic(er)
		}
		defer file.Close()

		w := csv.NewWriter(file)
		title := []string{"txpool size", "tx", "ctx"}
		w.Write(title)
		w.Flush()
		w.Write(str)
		w.Flush()
	} else {
		file, err := os.OpenFile(targetPath, os.O_APPEND|os.O_RDWR, 0666)

		if err != nil {
			log.Panic(err)
		}
		defer file.Close()
		writer := csv.NewWriter(file)
		err = writer.Write(str)
		if err != nil {
			log.Panic()
		}
		writer.Flush()
	}

	f.Close()
}

// get the digest of request
func getDigest(r *message.Request) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		log.Panic(err)
	}
	hash := sha256.Sum256(b)
	return hash[:]
}
