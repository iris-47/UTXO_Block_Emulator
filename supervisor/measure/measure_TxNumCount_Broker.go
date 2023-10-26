package measure

import "blockEmulator/message"

// to test cross-transaction rate
type TestTxNumCount_Broker struct {
	epochID int
	txNum   []float64
}

func NewTestTxNumCount_Broker() *TestTxNumCount_Broker {
	return &TestTxNumCount_Broker{
		epochID: -1,
		txNum:   make([]float64, 0),
	}
}

func (ttnc *TestTxNumCount_Broker) OutputMetricName() string {
	return "Tx_number"
}

func (ttnc *TestTxNumCount_Broker) UpdateMeasureRecord(b *message.BlockInfoMsg) {
	if b.BlockBodyLength == 0 { // empty block
		return
	}
	epochid := b.Epoch
	// extend
	for ttnc.epochID < epochid {
		ttnc.txNum = append(ttnc.txNum, 0)
		ttnc.epochID++
	}

	ttnc.txNum[epochid] += float64(len(b.ExcutedTxs)) + (float64(b.Broker1TxNum)+float64(b.Broker2TxNum))/2
}

func (ttnc *TestTxNumCount_Broker) HandleExtraMessage([]byte) {}

func (ttnc *TestTxNumCount_Broker) OutputRecord() (perEpochCTXs []float64, totTxNum float64) {
	perEpochCTXs = make([]float64, 0)
	totTxNum = 0.0
	for _, tn := range ttnc.txNum {
		perEpochCTXs = append(perEpochCTXs, tn)
		totTxNum += tn
	}
	return perEpochCTXs, totTxNum
}
