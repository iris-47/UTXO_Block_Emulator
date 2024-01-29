package params

var (
	MaxLatency          = 5 // the maximum of network delay(ms)
	UTXO                = false
	UseSyncHotstuff     = false
	TxInputCount        = 20000
	Block_Interval      = 1000   // generate new block interval
	MaxBlockSize_global = 500    // the block contains the maximum number of transactions
	InjectSpeed         = 1000   // the transaction inject speed
	TotalDataSize       = 100000 // the total number of txs
	BatchSize           = 4000   // supervisor read a batch of txs then send them, it should be larger than inject speed
	BrokerNum           = 10
	NodesInShard        = 4
	ShardNum            = 4
	DataWrite_path      = "./result/"                                        // measurement data result output path
	LogWrite_path       = "./log"                                            // log output path
	SupervisorAddr      = "127.0.0.1:18800"                                  //supervisor ip address
	FileInput           = `../dataset/2000000to2999999_BlockTransaction.csv` //the raw BlockTransaction data path
)
