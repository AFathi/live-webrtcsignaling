package main

const channelSize = 1000

func GetSeqNumberWithoutCycles(seq uint64) uint16 {
	return uint16(seq % (uint64(MaxUint16) + 1))
}

func GetSeqNumberWithCycles(seq uint16, seqCycles uint32) uint64 {
	return uint64(seq) + uint64(seqCycles)*(uint64(MaxUint16)+1)
}

func GetTimestampWithoutCycles(timestamp uint64) uint32 {
	return uint32(timestamp % (uint64(MaxUint32) + 1))
}
