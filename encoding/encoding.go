package encoding

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"filippo.io/age"
)

const (
	MaxMessageLen = 10000
)

type MessageType uint64

const (
	PublicKey MessageType = iota + 1
	Text
)

// Packet structure:
// flags (8 bytes) - type, version
// signature // TODO ?
// ciphertext

type Encoder struct {
	ownPrivKey     *age.X25519Identity
	publicKeyBytes []byte
	peerPublicKey  *age.X25519Recipient
}

func NewEncoder(privateKey *age.X25519Identity) *Encoder {
	return &Encoder{
		ownPrivKey:     privateKey,
		publicKeyBytes: []byte(privateKey.Recipient().String()),
	}
}

func (e *Encoder) SetPeerPublicKey(publicKey []byte) error {
	pub, err := age.ParseX25519Recipient(string(publicKey))
	if err != nil {
		return err
	}
	e.peerPublicKey = pub

	return nil
}

func (e *Encoder) GetOwnPublicKey() []byte {
	return e.publicKeyBytes
}

func (e *Encoder) Copy() *Encoder {
	return &Encoder{
		ownPrivKey:     e.ownPrivKey,
		publicKeyBytes: e.publicKeyBytes,
		peerPublicKey:  e.peerPublicKey,
	}
}

func (e *Encoder) PackMessage(flags MessageType, message []byte) ([]byte, error) {
	// TODO: reuse buffers with sync.Pool, optimize allocations
	buf := &bytes.Buffer{}
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], uint64(flags))
	buf.Write(data[:])

	w, err := age.Encrypt(buf, e.peerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted stream: %v", err)
	}
	if _, err := w.Write(message); err != nil {
		return nil, fmt.Errorf("failed to write to encrypted stream: %v", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to flush to encrypted stream: %v", err)
	}

	return EncodeBase64(buf.Bytes()), nil
}

func (e *Encoder) UnpackMessage(encodedBody []byte) ([]byte, MessageType, error) {
	decoded, err := DecodeBase64(encodedBody)
	if err != nil {
		return nil, 0, err
	}

	if len(decoded) < 8 {
		return nil, 0, fmt.Errorf("invalid decoded message length, should be > 8, got %d", len(decoded))
	}
	flags := binary.BigEndian.Uint64(decoded[:8])
	r, err := age.Decrypt(bytes.NewReader(decoded[8:]), e.ownPrivKey)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open decrypted stream: %v", err)
	}
	out := &bytes.Buffer{}
	if _, err := io.Copy(out, r); err != nil {
		return nil, 0, fmt.Errorf("failed to read from decrypted stream: %v", err)
	}

	return out.Bytes(), MessageType(flags), nil
}

func GenerateKey() (*age.X25519Identity, error) {
	return age.GenerateX25519Identity()
}

func UnmarshalPrivateKey(key string) (*age.X25519Identity, error) {
	return age.ParseX25519Identity(key)
}

func UnmarshalPublicKey(key string) (*age.X25519Recipient, error) {
	return age.ParseX25519Recipient(key)
}

func EncodeBase64(data []byte) []byte {
	encodedBuf := make([]byte, base64.RawURLEncoding.EncodedLen(len(data)))
	base64.RawURLEncoding.Encode(encodedBuf, data)
	return encodedBuf
}

func DecodeBase64(encoded []byte) ([]byte, error) {
	decoded := make([]byte, base64.RawURLEncoding.DecodedLen(len(encoded)))
	ndst, err := base64.RawURLEncoding.Decode(decoded, encoded)
	if err != nil {
		return nil, fmt.Errorf("base64.Decode: %v", err)
	}
	return decoded[:ndst], nil
}
