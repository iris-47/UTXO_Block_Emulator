### 1. fix the issue that UTXO set speed too low
- UTXO TX Pool wait until full to return a cluster of Txs
- change the node end detection logic -- time
### 2. fix the bug that the supervisor stop the process when some shard still working


## Problem

- 网络中的时延远低于处理消息所需时间