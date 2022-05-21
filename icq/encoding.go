package icq

import (
	"github.com/pymq/demhack4/encoding"
)

// TODO этим пакетом только людей пугать. Надо убрать.

type ICQEncoder struct {
	encoding.Encoder
}

func (e *ICQEncoder) Encode(message []byte) ([]byte, error) {
	return e.PackMessage(1, message)
}

func (e *ICQEncoder) Decode(message []byte) ([]byte, error) {
	rMsg, _, err := e.UnpackMessage(message)
	return rMsg, err
}
