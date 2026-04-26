package xiaomi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := "my-secret-password-123!@#"

	ciphertext, err := Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	plaintext := "same-input"

	c1, err := Encrypt(plaintext)
	require.NoError(t, err)

	c2, err := Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, c1, c2, "random nonce should produce different ciphertexts")
}

func TestDecryptInvalidBase64(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!")
	assert.Error(t, err)
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	_, err := Decrypt("AAAAAAAAAA==")
	assert.Error(t, err)
}

func TestDeriveKeyConsistency(t *testing.T) {
	k1 := deriveKey()
	k2 := deriveKey()
	assert.Equal(t, k1, k2, "same machine should derive same key")
}
