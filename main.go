package main

import (
	"blockEmulator/build"
	"blockEmulator/params"

	"github.com/spf13/pflag"
)

var (
	shardNum        int
	nodeNum         int
	shardID         int
	nodeID          int
	modID           int
	isClient        bool
	isGen           bool
	useUTXO         bool
	useSyncHotstuff bool
)

func main() {
	pflag.IntVarP(&shardNum, "shardNum", "S", 2, "indicate that how many shards are deployed")
	pflag.IntVarP(&nodeNum, "nodeNum", "N", 4, "indicate how many nodes of each shard are deployed")
	pflag.IntVarP(&shardID, "shardID", "s", 0, "id of the shard to which this node belongs, for example, 0")
	pflag.IntVarP(&nodeID, "nodeID", "n", 0, "id of this node, for example, 0")
	pflag.IntVarP(&modID, "modID", "m", 3, "choice Committee Method,for example, 0, [CLPA_Broker,CLPA,Broker,Relay] ")
	pflag.BoolVarP(&isClient, "client", "c", false, "whether this node is a client")
	pflag.BoolVarP(&isGen, "gen", "g", false, "generation bat")
	pflag.BoolVarP(&useUTXO, "utxo", "u", false, "use utxo")
	pflag.BoolVarP(&useSyncHotstuff, "synchotstuff", "h", false, "use synchotstuff")
	pflag.Parse()

	if useUTXO {
		params.UTXO = true
	}

	if useSyncHotstuff {
		params.UseSyncHotstuff = true
	}

	if isGen {
		// build.GenerateBatFile(nodeNum, shardNum, modID, useUTXO)
		build.GenerateShellFile(nodeNum, shardNum, modID, useUTXO, useSyncHotstuff)
		return
	}

	if isClient {
		build.BuildSupervisor(uint64(nodeNum), uint64(shardNum), uint64(modID))
	} else {
		build.BuildNewNode(uint64(nodeID), uint64(nodeNum), uint64(shardID), uint64(shardNum), uint64(modID))
	}
}
