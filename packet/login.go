package packet

import (
	"github.com/google/uuid"
)

// @gen:r,w,regserver
type LoginStart struct {
	Name       string    `field:"String"`
	PlayerUUID uuid.UUID `field:"UUID"`
}

func (p LoginStart) ID() int32 {
	return 0
}

// @gen:r,w,regserver
type EncryptionResponse struct {
	SharedSecret []byte `field:"PrefixedArray" inner:"Byte"`
	VerifyToken  []byte `field:"PrefixedArray" inner:"Byte"`
}

func (p EncryptionResponse) ID() int32 {
	return 1
}

// @gen:r,w,regserver
type LoginAcknowledge struct{}

func (p LoginAcknowledge) ID() int32 {
	return 3
}

// @gen:r,w,regclient
type LoginDisconnect struct {
	Reason string `field:"String"` // JSON Text Component
}

func (p LoginDisconnect) ID() int32 {
	return 0
}

// @gen:r,w,regclient
type EncryptionRequest struct {
	ServerID    string `field:"String"`
	PublicKey   []byte `field:"PrefixedArray" inner:"Byte"`
	VerifyToken []byte `field:"PrefixedArray" inner:"Byte"`
	ShouldAuth  bool   `field:"Boolean"`
}

func (p EncryptionRequest) ID() int32 {
	return 1
}

type gameProfileProperty struct {
	Name      string
	Value     string
	Signature Optional[string]
}

func writeGameProfileProperty(w Writer, v gameProfileProperty) (err error) {
	if err = WriteString(w, v.Name); err != nil {
		return
	}
	if err = WriteString(w, v.Value); err != nil {
		return
	}
	err = WriteOptional(w, v.Signature, WriteString)
	return
}

func readGameProfileProperty(r Reader) (v gameProfileProperty, err error) {
	v.Name, err = ReadString(r)
	if err != nil {
		return
	}
	v.Value, err = ReadString(r)
	if err != nil {
		return
	}
	v.Signature, err = ReadOptional(r, ReadString)
	return
}

// @gen:r,w,regclient
type LoginSuccess struct {
	UUID              uuid.UUID             `field:"UUID"`
	Username          string                `field:"String"`
	Properties        []gameProfileProperty `field:"PrefixedArray" write:"writeGameProfileProperty" read:"readGameProfileProperty"`
	StrictErrHandling bool                  `field:"Boolean"`
}

func (p LoginSuccess) ID() int32 {
	return 2
}

// @gen:r,w,regclient
type SetCompression struct {
	Threshold int32 `field:"VarInt"`
}

func (p SetCompression) ID() int32 {
	return 3
}
