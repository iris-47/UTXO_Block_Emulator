UTXO实现方案，基本没有改变函数调用关系

## UTXO Transaction
UTXO Transaction交易结构体如下
```go
// TxIn defines a transaction input.
type TxIn struct {
	PrevTxId  []byte // 该TxIn引用的TxOut所属TxId
	Index     int    // 该TxIn引用的TxOut在所在Tx下的index
	Signature []byte // 未实现
	PubKey    []byte // 当前等效于地址
}

// TXOut represents a transaction output
type TxOut struct {
	Value      *big.Int // 金额
	PubKeyHash []byte   // PubKey的Sum256 Hash
}

// UTXOTransaction represents a Bitcoin transaction
type UTXOTransaction struct {
	TxId []byte    // Tx的标识符
	Vin  []TxIn    // Tx的Input
	Vout []TxOut   // Tx的Output

	Nonce int64    // 随机数，用于防止CoinbaseTx生成一样的TxId

	Time      time.Time // 进入交易池的时间，用于测试性能
	TxHash    []byte 
	Relayed   bool   // used in transaction relaying
	Recipient string // used in relay to indicate the receiver shard
}
```

## UTXO Set
仿BitCoin的方案，将当前区块链所有UTXO以\[Txid: []TxOutput\]的形式单独存储，便于查找。仅主节点维护UTXO Set，并负责生成UTXO Tx。
### FindSpendableOutputs()
遍历UTXO Set数据库中的tx，找出属于当前pubkeyHash的Output，直到满足需要的金额位置，返回找到的[]output和这些output金额之和
### AddTx2UTXOSet()
根据Tx来更新UTXO Set, 遍历Tx的所有Input，将这些Input引用的Output从UTXO中移除，并将Tx的所有Output加入到UTXO Set。对于Coinbase交易则只需要遍历Output即可

## BlockChain
该部分主要修改一些接口，并新增[NewUTXOTransaction()](#NewUTXOTransaction())函数
### NewUTXOTransaction()
这个函数放在Chain下是因为其需要调用UTXO Set来生成新的交易，因此不能放在Core下
该函数调用[FindSpendableOutputs()](#FindSpendableOutputs())来查找可用的Output，如果返回的Output为空，则生成一个新的Coinbase交易以，并调用[AddTx2UTXOSet()](#AddTx2UTXOSet())将Coinbase交易加入UTXO Set数据库，再次调用[FindSpendableOutputs()](#FindSpendableOutputs())来找到刚才那笔coinbase，该coinbase金额设置为刚好满足本次交易，以免堆积过多的交易造成查找缓慢。随后生成UTXOTransaction并返回，如果生成了新的Coinbase也一并返回。

## PBFT Outside
在生成Tx阶段，没有修改Supervisor，而是选择修改主节点的PBFT Outside模块，由主节点生成新的UTXO Tx。
### handleInjectTx()
该函数接收的仍然是Account格式的Txs包，调用[NewUTXOTransaction()](#NewUTXOTransaction())来生成新的UTXO Tx并加入交易池，如果同时生成了CoinbaseTx，则将CoinbaseTx一并加入交易池等待打包上链。
### handleRelay()
该函数生成一个新的Coinbase交易，以实现跨分片交易

## PBFT Inside
改动不大，主要改动Commit阶段
### HandleinCommit()
将区块上链，主节点负责将需要Relay的Tx打包，Coinbase不可能跨片因此略过。接下来的向Supervisor发送bloackMsg中保留了原版的transaction结构，因为Supervisor其实只需要Tx.Hash和Tx.Time，所以在这里直接将这两个字段从UTXO Tx复制到Tx即可发送。