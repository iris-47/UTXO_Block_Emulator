package synchotstuff_all

import (
	"blockEmulator/chain"
	node_log "blockEmulator/consensus_shard/nodelogger"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/shard"
	"bufio"
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

type SyncHotstuffConsensusNode struct {
	// the local config about pbft
	RunningNode *shard.Node // the node information
	ShardID     uint64      // denote the ID of the shard (or pbft), only one pbft consensus in a shard
	NodeID      uint64      // denote the ID of the node in the pbft (shard)

	// the data structure for blockchain
	CurChain *chain.BlockChain // all node in the shard maintain the same blockchain
	db       ethdb.Database    // to save the mpt

	// the global config about pbft
	ChainConfig    *params.ChainConfig          // the chain config
	ip_nodeTable   map[uint64]map[uint64]string // denote the ip of the specific node
	node_nums      uint64                       // the number of nodes in this pfbt, denoted by N
	malicious_nums uint64                       // f, 3f + 1 = N
	view           uint64                       // denote the view of this pbft, the main node can be inferred from this variant

	// the control message and message checking utils in pbft
	sequenceID     uint64                      // the message sequence id of the pbft
	stop           bool                        // send stop signal
	pStop          chan uint64                 // channle for stopping consensus
	pBreak         chan uint64                 // channel to interrupt the commit waiting process
	requestPool    map[string]*message.Request // RequestHash to Request
	isVoteBordcast map[string]bool             // denote whether the vote is broadcast
	isReply        map[string]bool             // denote whether the message is reply
	height2Digest  map[uint64]string           // sequence (block height) -> request, fast read

	// locks about pbft
	sequenceLock sync.Mutex // the lock of sequence
	lock         sync.Mutex // lock the stage
	askForLock   sync.Mutex // lock for asking for a serise of requests
	stopLock     sync.Mutex // lock the stop varient

	// seqID of other Shards, to synchronize
	seqIDMap   map[uint64]uint64
	seqMapLock sync.Mutex

	// logger
	pl *node_log.NodeLog
	// tcp control
	tcpln       net.Listener
	tcpPoolLock sync.Mutex

	//to handle the message in the consenses
	ihm SyncHotstuffInsideExtraHandleMod

	//to handle the message outside of consenses
	ohm SyncHotstuffOutsideHandleMod
}

// generate a pbft consensus for a node
func NewSyncHotstuffNode(shardID, nodeID uint64, pcc *params.ChainConfig, messageHandleType string) *SyncHotstuffConsensusNode {
	node := new(SyncHotstuffConsensusNode)
	node.ip_nodeTable = params.IPmap_nodeTable
	node.node_nums = pcc.Nodes_perShard
	node.ShardID = shardID
	node.NodeID = nodeID
	node.ChainConfig = pcc
	fp := "./record/ldb/s" + strconv.FormatUint(shardID, 10) + "/n" + strconv.FormatUint(nodeID, 10)
	var err error
	node.db, err = rawdb.NewLevelDBDatabase(fp, 0, 1, "accountState", false)
	if err != nil {
		log.Panic(err)
	}
	node.CurChain, err = chain.NewBlockChain(pcc, node.db)
	if err != nil {
		log.Panic("cannot new a blockchain")
	}

	node.RunningNode = &shard.Node{
		NodeID:  nodeID,
		ShardID: shardID,
		IPaddr:  node.ip_nodeTable[shardID][nodeID],
	}

	node.stop = false
	node.sequenceID = node.CurChain.CurrentBlock.Header.Number + 1
	node.pStop = make(chan uint64)
	node.requestPool = make(map[string]*message.Request)
	node.isVoteBordcast = make(map[string]bool)
	node.isReply = make(map[string]bool)
	node.height2Digest = make(map[uint64]string)
	node.malicious_nums = (node.node_nums) / 2
	node.view = 0

	node.seqIDMap = make(map[uint64]uint64)

	node.pl = node_log.NewLogger(shardID, nodeID)

	// choose how to handle the messages in pbft or beyond pbft
	switch string(messageHandleType) {
	default:
		if params.UTXO {
			node.ihm = &RawUTXORelayExtraHandleMod{
				node: node,
			}
			node.ohm = &RawUTXORelayOutsideModule{
				node: node,
			}
		}
		// else {
		// 	// p.ihm = &RawRelayPbftExtraHandleMod{
		// 	// 	pbftNode: p,
		// 	// }
		// 	// p.ohm = &RawRelayOutsideModule{
		// 	// 	pbftNode: p,
		// 	// }
		// }
	}

	return node
}

// handle the raw message, send it to corresponded interfaces
func (node *SyncHotstuffConsensusNode) handleMessage(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	// pbft inside message type
	case message.CPropose:
		node.handlePropose(content)
	case message.CVote:
		node.handleVote(content)
	case message.CRequestOldrequest:
		node.handleRequestOldSeq(content)
	case message.CSendOldrequest:
		node.handleSendOldSeq(content)
	case message.CStop:
		node.WaitToStop()

	// handle the message from outside
	default:
		node.ohm.HandleMessageOutsideConsensus(msgType, content)
	}
}

func (node *SyncHotstuffConsensusNode) handleClientRequest(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		if node.getStopSignal() {
			return
		}
		switch err {
		case nil:
			node.tcpPoolLock.Lock()
			node.handleMessage(clientRequest)
			node.tcpPoolLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

func (node *SyncHotstuffConsensusNode) TcpListen() {
	ln, err := net.Listen("tcp", node.RunningNode.IPaddr)
	node.tcpln = ln
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := node.tcpln.Accept()
		if err != nil {
			return
		}
		go node.handleClientRequest(conn)
	}
}

// when received stop
func (node *SyncHotstuffConsensusNode) WaitToStop() {
	node.pl.Nlog.Println("handling stop message")
	node.stopLock.Lock()
	node.stop = true
	node.stopLock.Unlock()
	if node.NodeID == node.view {
		node.pStop <- 1
	}
	networks.CloseAllConnInPool()
	node.tcpln.Close()
	node.closePbft()
	node.pl.Nlog.Println("handled stop message")
}

func (node *SyncHotstuffConsensusNode) getStopSignal() bool {
	node.stopLock.Lock()
	defer node.stopLock.Unlock()
	return node.stop
}

// close the pbft
func (node *SyncHotstuffConsensusNode) closePbft() {
	node.CurChain.CloseBlockChain()
}
