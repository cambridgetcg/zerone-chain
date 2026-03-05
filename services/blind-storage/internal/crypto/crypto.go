package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// GenerateMasterKey generates a random 256-bit AES master key.
func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	return key, nil
}

// DeriveChunkKey derives a per-chunk key from the master key using HKDF.
func DeriveChunkKey(masterKey []byte, chunkIndex int, datasetVersion string) ([]byte, error) {
	info := make([]byte, 4+len(datasetVersion))
	binary.BigEndian.PutUint32(info[:4], uint32(chunkIndex))
	copy(info[4:], datasetVersion)

	hkdfReader := hkdf.New(sha256.New, masterKey, nil, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("derive chunk key: %w", err)
	}
	return key, nil
}

// EncryptChunk encrypts a chunk using AES-256-GCM.
// Returns: ciphertext (nonce prepended).
func EncryptChunk(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptChunk decrypts an AES-256-GCM encrypted chunk.
// Expects nonce prepended to ciphertext.
func DecryptChunk(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// HashChunk returns the SHA-256 hash of a chunk.
func HashChunk(data []byte) [32]byte {
	return sha256.Sum256(data)
}
