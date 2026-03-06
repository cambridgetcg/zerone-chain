package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

// Authenticator handles attestation exchange and session key derivation
// between a TEE enclave and validators during shard collection.
type Authenticator struct {
	EnclaveID   string
	EnclaveKey  *ecdsa.PrivateKey
	Attestation []byte
}

// HelloMessage is the initial message from TEE to validator.
type HelloMessage struct {
	EnclaveID   string
	Attestation []byte
	PublicKey    *ecdsa.PublicKey
}

// ChallengeResponse is the TEE's signed response to a validator challenge.
type ChallengeResponse struct {
	Signature  []byte
	SessionKey []byte // derived via ECDH
}

// Hello produces the initial attestation presentation for a validator.
func (a *Authenticator) Hello() *HelloMessage {
	return &HelloMessage{
		EnclaveID:   a.EnclaveID,
		Attestation: a.Attestation,
		PublicKey:    &a.EnclaveKey.PublicKey,
	}
}

// RespondToChallenge signs the validator's challenge nonce and derives
// a shared session key via ECDH with the validator's public key.
func (a *Authenticator) RespondToChallenge(challenge []byte, validatorPub *ecdsa.PublicKey) (*ChallengeResponse, error) {
	sig, err := Sign(a.EnclaveKey, challenge)
	if err != nil {
		return nil, err
	}

	sessionKey, err := DeriveSessionKey(a.EnclaveKey, validatorPub)
	if err != nil {
		return nil, err
	}

	return &ChallengeResponse{
		Signature:  sig,
		SessionKey: sessionKey,
	}, nil
}

// DeriveSessionKey performs ECDH key agreement and derives a 32-byte
// AES-256-GCM session key using SHA-256 over the shared secret.
func DeriveSessionKey(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey) ([]byte, error) {
	// ECDH: multiply peer's public point by our private scalar
	x, _ := pub.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	if x == nil {
		return nil, errors.New("ECDH key agreement failed")
	}

	// Derive 32-byte key via SHA-256 of the shared x-coordinate
	h := sha256.Sum256(x.Bytes())
	return h[:], nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns nonce || ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
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

// Decrypt decrypts AES-256-GCM ciphertext (nonce || ciphertext).
func Decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateNonce returns a 32-byte cryptographically random nonce.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// Sign signs data with an ECDSA private key (SHA-256 hash + ASN.1 DER signature).
func Sign(priv *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return nil, err
	}
	// Encode r || s as fixed-size 64-byte signature (P-256)
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return sig, nil
}

// Verify checks an ECDSA signature against data and a public key.
func Verify(pub *ecdsa.PublicKey, data, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	return ecdsa.Verify(pub, hash[:], r, s)
}

// EncryptShard encrypts a ShardResponse's TDU data for transport.
func EncryptShard(key []byte, resp *ShardResponse) ([]byte, error) {
	var buf []byte
	for _, tdu := range resp.TDUs {
		buf = append(buf, tdu.Content...)
	}
	return Encrypt(key, buf)
}
