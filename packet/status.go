package packet

// @gen:r,w,regserver
type StatusReqPacket struct{}

func (p StatusReqPacket) ID() int32 {
	return 0
}

// @gen:r,w,regserver
type PingReqPacket struct {
	Timestamp int64 `field:"Long"`
}

func (p PingReqPacket) ID() int32 {
	return 1
}

// @gen:r,w,regclient
type StatusRespPacket struct {
	Response string `field:"String"`
}

func (p StatusRespPacket) ID() int32 {
	return 0
}
