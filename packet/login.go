package packet

import (
	"io"

	"github.com/google/uuid"
)

// @gen
type LoginStart struct {
	Name       string    `field:"String"`
	PlayerUUID uuid.UUID `field:"UUID"`
}

func (p LoginStart) ID() int32 {
	return 0
}

// @gen
type EncryptionResponse struct {
	SharedSecret []byte `field:"PrefixedArray" inner:"Byte"`
	VerifyToken  []byte `field:"PrefixedArray" inner:"Byte"`
}

func (p EncryptionResponse) ID() int32 {
	return 1
}

// @gen
type LoginAcknowledge struct{}

func (p LoginAcknowledge) ID() int32 {
	return 3
}

// @gen
type LoginDisconnect struct {
	Reason string `field:"String"` // JSON Text Component
}

func (p LoginDisconnect) ID() int32 {
	return 0
}

// @gen
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

func writeGameProfileProperty(w io.Writer, v gameProfileProperty) (err error) {
	if err = WriteString(w, v.Name); err != nil {
		return
	}
	if err = WriteString(w, v.Value); err != nil {
		return
	}
	err = WriteOptional(w, v.Signature, WriteString)
	return
}

func readGameProfileProperty(r io.Reader) (v gameProfileProperty, err error) {
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

// @gen
type LoginSuccess struct {
	UUID              uuid.UUID             `field:"UUID"`
	Username          string                `field:"String"`
	Properties        []gameProfileProperty `field:"PrefixedArray" write:"writeGameProfileProperty" read:"readGameProfileProperty"`
	StrictErrHandling bool                  `field:"Boolean"`
}

func (p LoginSuccess) ID() int32 {
	return 2
}

// @gen
type SetCompression struct {
	Threshold int32 `field:"VarInt"`
}

func (p SetCompression) ID() int32 {
	return 3
}
