1. Bitcoin or Ethereum? 

   - Ethereum：[sender, receiver, val]，由平台生成TxIn和TxOut
   - Bitcoin: 有TxId，TxIn和TxOut等

   

2. 余额不足怎么处理，由于数据集从中间开始，会有很多这样的情况。Accout可以每新增一个账户给那个账户许多启示资金，但UTXO不会记录账户。

   - 如果可用UTXO为空则新建一笔coinbase，有UTXO但不足以支付则按余额不足处理
   - 按照余额不足处理
   - 新增一个账户记录数据库，如果发现是新增的账户就给一笔coinbase
   - 数据集从第一个区块开始

3. 如何识别一个接收者的分片？TxOut以PubkeyHash的方式标志所有者，不能用于识别接收者

   - 新增一个字段

4. UTXO Set只能放在Commitee？但生成RelayTx的时候

5. 在PBFT_Outside模块中生成Txs是否会影响整个系统的性能？

6. 如何改进UTXO Set，改成类似于StatusTrie那样？


