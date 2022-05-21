package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncoding(t *testing.T) {
	privOne, _, err := GenerateKey()
	assert.NoError(t, err)
	encOne, err := NewEncoder(privOne)
	assert.NoError(t, err)

	privTwo, _, err := GenerateKey()
	assert.NoError(t, err)
	encTwo, err := NewEncoder(privTwo)
	assert.NoError(t, err)

	err = encOne.SetPeerPublicKey(encTwo.GetOwnPublicKey())
	assert.NoError(t, err)
	err = encTwo.SetPeerPublicKey(encOne.GetOwnPublicKey())
	assert.NoError(t, err)

	const expectedText = "hello world!"

	encodedMessage, err := encOne.PackMessage(PublicKey, []byte(expectedText))
	assert.NoError(t, err)
	decodedMessage, flags, err := encTwo.UnpackMessage(encodedMessage)
	assert.NoError(t, err)
	assert.Equal(t, PublicKey, flags)
	assert.Equal(t, expectedText, string(decodedMessage))
}
