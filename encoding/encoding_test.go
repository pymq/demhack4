package encoding

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncoding(t *testing.T) {
	encOne, encTwo := setupTwoEncoders(t)
	const expectedText = "hello world!"

	encodedMessage, err := encOne.PackMessage(PublicKey, []byte(expectedText))
	assert.NoError(t, err)
	decodedMessage, flags, err := encTwo.UnpackMessage(encodedMessage)
	assert.NoError(t, err)
	assert.Equal(t, PublicKey, flags)
	assert.Equal(t, expectedText, string(decodedMessage))
}

func BenchmarkEncodingSize(b *testing.B) {
	rnd := rand.New(rand.NewSource(42))

	genMessage := func(size int) []byte {
		data := make([]byte, 0, size)
		for i := 0; i < size; i++ {
			data = append(data, byte(rnd.Intn(256)))
		}
		return data
	}

	encoder, _ := setupTwoEncoders(b)

	messageSizes := []int{50, 100, 300, 440, 800, 1800, 3000, 6000, 10000}
	for _, msgSize := range messageSizes {
		b.Run(fmt.Sprintf("%d bytes message", msgSize), func(b *testing.B) {
			message := genMessage(msgSize)
			encodedMessage, err := encoder.PackMessage(Text, message)
			assert.NoError(b, err)

			b.ReportMetric(0, "ns/op") // disable metric
			b.ReportMetric(float64(len(encodedMessage)), "encoded")
			b.ReportMetric(float64(base64.RawURLEncoding.EncodedLen(len(message))), "base64")
			b.ReportMetric(float64(len(message)), "original")
			b.ReportMetric(float64(len(encodedMessage))/float64(len(message)), "ratio")
		})
	}
}

func setupTwoEncoders(t testing.TB) (*Encoder, *Encoder) {
	privOne, err := GenerateKey()
	assert.NoError(t, err)
	encOne := NewEncoder(privOne)

	privTwo, err := GenerateKey()
	assert.NoError(t, err)
	encTwo := NewEncoder(privTwo)

	err = encOne.SetPeerPublicKey(encTwo.GetOwnPublicKey())
	assert.NoError(t, err)
	err = encTwo.SetPeerPublicKey(encOne.GetOwnPublicKey())
	assert.NoError(t, err)

	return encOne, encTwo
}
