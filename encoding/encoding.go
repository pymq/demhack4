package encoding

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	rsaKeySize       = 2048
	rsaSignatureSize = 256
)

type MessageType uint64

const (
	PublicKey MessageType = iota + 1
	Text
)

var rsaLabel = []byte("")

// Packet structure:
// flags (8 bytes) - type, version
// signature // TODO ?
// ciphertext

type Encoder struct {
	ownPrivKey     *rsa.PrivateKey
	publicKeyBytes []byte
	peerPublicKey  *rsa.PublicKey
}

func NewEncoder(privateKey *rsa.PrivateKey) (*Encoder, error) {
	_, publicBytes, err := MarshalKey(privateKey)
	if err != nil {
		return nil, err
	}
	return &Encoder{
		ownPrivKey:     privateKey,
		publicKeyBytes: publicBytes,
	}, nil
}

func (e *Encoder) SetPeerPublicKey(publicKey []byte) error {
	pub, err := x509.ParsePKCS1PublicKey(publicKey)
	if err != nil {
		return err
	}
	e.peerPublicKey = pub

	return nil
}

func (e *Encoder) GetOwnPublicKey() []byte {
	return e.publicKeyBytes
}

func (e *Encoder) PackMessage(flags MessageType, message []byte) ([]byte, error) {
	// TODO: reuse buffers with sync.Pool, optimize allocations
	buf := bytes.Buffer{}
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], uint64(flags))
	buf.Write(data[:])

	// TODO: check this fact
	// The message must be no longer than the length of the public modulus minus twice the hash length, minus a further 2.
	ciphertext, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		e.peerPublicKey,
		message,
		rsaLabel,
	)
	if err != nil {
		return nil, fmt.Errorf("rsa.EncryptOAEP: %v", err)
	}
	buf.Write(ciphertext)

	return EncodeBase64(buf.Bytes()), nil
}

func (e *Encoder) UnpackMessage(encodedBody []byte) ([]byte, MessageType, error) {
	decoded, err := DecodeBase64(encodedBody)
	if err != nil {
		return nil, 0, err
	}

	flags := binary.BigEndian.Uint64(decoded[:8])
	message, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		e.ownPrivKey,
		decoded[8:],
		rsaLabel,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("rsa.DecryptOAEP: %v", err)
	}

	return message, MessageType(flags), nil
}

func GenerateKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	private, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, err
	}
	return private, &private.PublicKey, nil
}

func MarshalKey(private *rsa.PrivateKey) (privateBytes, publicBytes []byte, err error) {
	public := &private.PublicKey
	priBytes, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, nil, err
	}
	pubBytes := x509.MarshalPKCS1PublicKey(public)

	return priBytes, pubBytes, nil
}

func UnmarshalPrivateKeyWithBase64(key []byte) (*rsa.PrivateKey, error) {
	decoded, err := DecodeBase64(key)
	if err != nil {
		return nil, err
	}

	pri, err := x509.ParsePKCS8PrivateKey(decoded)
	if err != nil {
		return nil, err
	}
	p, ok := pri.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("invalid private key")
	}
	return p, nil
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
