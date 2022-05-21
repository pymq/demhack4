package encoding

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/ascii85"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	rsaKeySize       = 2048
	rsaSignatureSize = 256
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

func (e *Encoder) PackMessage(flags uint64, message []byte) ([]byte, error) {
	// TODO: reuse buffers with sync.Pool, optimize allocations
	buf := bytes.Buffer{}
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], flags)
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

	encodedBuf := make([]byte, ascii85.MaxEncodedLen(buf.Len()))
	n := ascii85.Encode(encodedBuf, buf.Bytes())
	encodedBuf = encodedBuf[0:n]

	return encodedBuf, nil
}

func (e *Encoder) UnpackMessage(encodedBody []byte) ([]byte, uint64, error) {
	decoded := make([]byte, 4*len(encodedBody))
	ndst, nsrc, err := ascii85.Decode(decoded, encodedBody, true)
	if err != nil {
		return nil, 0, fmt.Errorf("ascii85.Decode: %v", err)
	} else if len(encodedBody) != nsrc {
		// should not happen in practice
		return nil, 0, fmt.Errorf("ascii85.Decode: mismatch between encoded len %d and decoded len %d", len(encodedBody), nsrc)
	}
	decoded = decoded[:ndst]

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

	return message, flags, nil
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

// TODO: use base85 to store in config
func UnmarshalPrivateKey(key []byte) (*rsa.PrivateKey, error) {
	pri, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	p, ok := pri.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("invalid private key")
	}
	return p, nil
}
