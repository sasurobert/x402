package multiversx

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

// VerifyUniversalSignature verifies the payment payload signature
// For MultiversX, this implies:
// 1. Validating the Ed25519 signature against the transaction bytes (if accessible/reconstructible).
// 2. Simulating the transaction (Smart Contract wallets, or just general validity).
//
// Since we don't effectively reconstruct the canonical JSON bytes locally easily without SDK canonicalizer,
// we rely heavily on Simulation for the cryptographic proof (the node verifies the sig).
//
// However, if we CAN verify Ed25519 locally, we should.
// But without the exact serialization logic from SDK, local verification is error-prone.
// EVM has standard hashing (EIP-712). MultiversX has "canonical JSON of fields".
// Recommendation: We stick to Simulation as the "Universal" verifier for MultiversX in this Go integration,
// because implementing a Go Canonical JSON Serializer for MultiversX txs perfectly matching the node is complex.
//
// But we will expose a function that integrates the checks.

func VerifyPayment(ctx context.Context, payload ExactRelayedPayload, requirements types.PaymentRequirements, simulator func(ExactRelayedPayload) (string, error)) (bool, error) {
	// 1. Static Checks
	if payload.Data.Receiver != requirements.PayTo {
		// Just a warning or strict check?
		// EVM checks strictness usually.
		return false, x402.NewVerifyError("receiver_mismatch", payload.Data.Sender, "multiversx", fmt.Errorf("got %s, want %s", payload.Data.Receiver, requirements.PayTo))
	}

	// 2. Signature Presence
	if payload.Data.Signature == "" {
		return false, x402.NewVerifyError(x402.ErrCodeSignatureInvalid, payload.Data.Sender, "multiversx", fmt.Errorf("missing signature"))
	}

	// 3. Local Ed25519 Verification
	// If we can verify locally, we essentially validate the signature is correct for the Sender.
	// But we also need to ensure the Tx itself is valid (nonce, balance, etc).
	// Simulator does both.
	// However, usually we trust the signature if we trust the sender has funds (which we can check separately or rely on error later).
	// For "VerifyPayment", getting a valid signature is a strong signal.

	// A. Construct Signable Message
	txData := struct {
		Nonce    uint64 `json:"nonce"`
		Value    string `json:"value"`
		Receiver string `json:"receiver"`
		Sender   string `json:"sender"`
		GasPrice uint64 `json:"gasPrice"`
		GasLimit uint64 `json:"gasLimit"`
		Data     string `json:"data"`
		ChainID  string `json:"chainID"`
		Version  uint32 `json:"version"`
		Options  uint32 `json:"options"`
	}{
		Nonce:    payload.Data.Nonce,
		Value:    payload.Data.Value,
		Receiver: payload.Data.Receiver,
		Sender:   payload.Data.Sender,
		GasPrice: payload.Data.GasPrice,
		GasLimit: payload.Data.GasLimit,
		Data:     payload.Data.Data,
		ChainID:  payload.Data.ChainID,
		Version:  payload.Data.Version,
		Options:  payload.Data.Options,
	}

	msgBytes, err := SerializeTransaction(txData)
	if err != nil {
		// If serialization fails, maybe fallback to sim?
		// But basic serialization shouldn't fail.
		return false, x402.NewVerifyError("serialization_failed", payload.Data.Sender, "multiversx", err)
	}

	// B. Verify Signature
	// Decode Sender Bech32 -> PubKey
	// address = hrp + pubkey
	_, pubKeyBytes, err := DecodeBech32(payload.Data.Sender)
	if err != nil {
		// Invalid sender address format
		return false, x402.NewVerifyError("invalid_sender_address", payload.Data.Sender, "multiversx", err)
	}

	sigBytes, err := hex.DecodeString(payload.Data.Signature)
	if err != nil {
		return false, x402.NewVerifyError("invalid_signature_hex", payload.Data.Sender, "multiversx", err)
	}

	if len(sigBytes) != 64 {
		return false, x402.NewVerifyError("invalid_signature_length", payload.Data.Sender, "multiversx", fmt.Errorf("expected 64 bytes, got %d", len(sigBytes)))
	}

	if len(pubKeyBytes) != 32 {
		return false, x402.NewVerifyError("invalid_public_key_length", payload.Data.Sender, "multiversx", fmt.Errorf("expected 32 bytes, got %d", len(pubKeyBytes)))
	}

	if ed25519.Verify(pubKeyBytes, msgBytes, sigBytes) {
		// Valid Signature!
		return true, nil
	}

	// 4. Fallback to Simulation
	// If local verify fails, it MIGHT be because `SerializeTransaction` doesn't match node's expectation
	// or it's a Smart Contract Wallet (MultiSig) which doesn't sign with Ed25519 of the address key.
	// So we attempt simulation.

	hash, err := simulator(payload)
	if err != nil {
		return false, x402.NewVerifyError("simulation_failed", payload.Data.Sender, "multiversx", err)
	}

	if hash == "" {
		return false, x402.NewVerifyError("simulation_returned_empty_hash", payload.Data.Sender, "multiversx", nil)
	}

	return true, nil
}
