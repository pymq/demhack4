package icq

import (
	"github.com/pymq/demhack4/encoding"
)

// TODO этим пакетом только людей пугать. Надо убрать.

type ICQEncoder struct {
	encoding.Encoder
}

func (e *ICQEncoder) Encode(message []byte) ([]byte, error) {
	return e.PackMessage(encoding.Text, message)
}

func (e *ICQEncoder) Decode(message []byte) ([]byte, encoding.MessageType, error) {
	return e.UnpackMessage(message)
}
