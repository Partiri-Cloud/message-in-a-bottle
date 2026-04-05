package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() []byte {
	key := make([]byte, 32)
	rand.Read(key)
	return key
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := validKey()
	plaintext := []byte("hello world, this is a secret message")

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDifferentCiphertexts(t *testing.T) {
	key := validKey()
	plaintext := []byte("same input")

	ct1, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	ct2, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	assert.NotEqual(t, ct1, ct2, "same plaintext should produce different ciphertexts due to random nonce")
}

func TestEncryptInvalidKeySize(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecryptInvalidKeySize(t *testing.T) {
	_, err := Decrypt([]byte("data"), []byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	key := validKey()
	_, err := Decrypt([]byte("tiny"), key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	key := validKey()
	ct, err := Encrypt([]byte("secret"), key)
	require.NoError(t, err)

	// Flip a byte in the ciphertext (after nonce)
	ct[len(ct)-1] ^= 0xff

	_, err = Decrypt(ct, key)
	assert.Error(t, err)
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := validKey()
	key2 := validKey()

	ct, err := Encrypt([]byte("secret"), key1)
	require.NoError(t, err)

	_, err = Decrypt(ct, key2)
	assert.Error(t, err)
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	key := validKey()

	ct, err := Encrypt([]byte{}, key)
	require.NoError(t, err)

	decrypted, err := Decrypt(ct, key)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}
