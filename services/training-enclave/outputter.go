package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

// SignedOutputs holds the enclave-signed outputs that exit the TEE.
type SignedOutputs struct {
	// Model weights signature.
	ModelWeightsSignature []byte `json:"model_weights_signature"`

	// Training metrics signature (covers TDU losses + aggregate metrics).
	MetricsSignature []byte `json:"metrics_signature"`

	// Benchmark results signature.
	BenchmarkSignature []byte `json:"benchmark_signature"`
}

// Outputter handles model export and signing with the enclave key.
type Outputter struct {
	EnclaveID  string
	EnclaveKey *ecdsa.PrivateKey
}

// SignOutputs signs all training outputs with the enclave key.
// Only signed outputs are permitted to leave the enclave.
func (o *Outputter) SignOutputs(result *TrainingResult) (*SignedOutputs, error) {
	if result == nil {
		return nil, errors.New("no training result to sign")
	}
	if o.EnclaveKey == nil {
		return nil, errors.New("enclave key not available")
	}

	// Sign model weights.
	modelSig, err := signData(o.EnclaveKey, result.ModelWeights)
	if err != nil {
		return nil, err
	}

	// Sign training metrics (hash of TDU losses + final loss).
	metricsHash := computeMetricsHash(result)
	metricsSig, err := signData(o.EnclaveKey, metricsHash)
	if err != nil {
		return nil, err
	}

	// Sign benchmark results.
	benchHash := computeBenchmarkHash(result)
	benchSig, err := signData(o.EnclaveKey, benchHash)
	if err != nil {
		return nil, err
	}

	return &SignedOutputs{
		ModelWeightsSignature: modelSig,
		MetricsSignature:      metricsSig,
		BenchmarkSignature:    benchSig,
	}, nil
}

// VerifyOutputSignature verifies an output signature against the enclave's public key.
func VerifyOutputSignature(pub *ecdsa.PublicKey, data, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	return ecdsa.Verify(pub, hash[:], r, s)
}

// signData signs data with an ECDSA private key, returning a 64-byte r||s signature.
func signData(priv *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return nil, err
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return sig, nil
}

// computeMetricsHash computes a SHA-256 hash over training metrics.
func computeMetricsHash(result *TrainingResult) []byte {
	h := sha256.New()
	for id, loss := range result.TDULosses {
		h.Write([]byte(id))
		buf := make([]byte, 8)
		bits := uint64(loss * 1e6)
		for i := 7; i >= 0; i-- {
			buf[i] = byte(bits & 0xff)
			bits >>= 8
		}
		h.Write(buf)
	}
	sum := h.Sum(nil)
	return sum
}

// computeBenchmarkHash computes a SHA-256 hash over benchmark results.
func computeBenchmarkHash(result *TrainingResult) []byte {
	h := sha256.New()
	buf := make([]byte, 8)
	bits := uint64(result.BenchmarkScore * 1e6)
	for i := 7; i >= 0; i-- {
		buf[i] = byte(bits & 0xff)
		bits >>= 8
	}
	h.Write(buf)
	for k, v := range result.BenchmarkDetail {
		h.Write([]byte(k))
		bits = uint64(v * 1e6)
		for i := 7; i >= 0; i-- {
			buf[i] = byte(bits & 0xff)
			bits >>= 8
		}
		h.Write(buf)
	}
	sum := h.Sum(nil)
	return sum
}
