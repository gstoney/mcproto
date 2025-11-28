package packet

// @gen
type StatusReqPacket struct{}

func (p StatusReqPacket) ID() int32 {
	return 0
}

// @gen
type PingReqPacket struct {
	Timestamp int64 `field:"Long"`
}

func (p PingReqPacket) ID() int32 {
	return 1
}

// @gen
type StatusRespPacket struct {
	Response string `field:"String"`
}

func (p StatusRespPacket) ID() int32 {
	return 0
}
