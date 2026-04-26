package xiaomi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/hkdf"
)

// Encrypt encrypts plaintext using AES-256-GCM with a machine-derived key.
// The returned string is base64-encoded (nonce + ciphertext + tag).
func Encrypt(plaintext string) (string, error) {
	key := deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
func Decrypt(ciphertext string) (string, error) {
	key := deriveKey()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("xiaomi: ciphertext too short")
	}

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// deriveKey produces a deterministic 32-byte key from machine-specific info
// using HKDF-SHA256. The key is the same on every call on the same machine.
func deriveKey() []byte {
	info := getMachineInfo()

	hkdfReader := hkdf.New(sha256.New, info, []byte("go2rtc-xiaomi"), []byte("password-encryption"))

	key := make([]byte, 32)
	// HKDF-SHA256 is deterministic and never errors; panic on unexpected failure
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		panic("hkdf: unexpected read error: " + err.Error())
	}
	return key
}

func getMachineInfo() []byte {
	hostname, _ := os.Hostname()
	mac := getFirstMAC()
	return append([]byte(hostname), mac...)
}

func getFirstMAC() []byte {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		hw := iface.HardwareAddr
		if len(hw) == 0 {
			continue
		}
		allZero := true
		for _, b := range hw {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}
		return hw
	}
	return nil
}
