package synchotstuff_all

import "blockEmulator/message"

type SyncHotstuffInsideExtraHandleMod interface {
	HandleinPropose(*message.Propose) bool
	Commit(*message.Propose) bool
	HandleinVote(*message.Vote) bool
	HandleReqestforOldSeq(*message.RequestOldMessage) bool
	HandleforSequentialRequest(*message.SendOldMessage) bool
}

type SyncHotstuffOutsideHandleMod interface {
	HandleMessageOutsideConsensus(message.MessageType, []byte) bool
}
