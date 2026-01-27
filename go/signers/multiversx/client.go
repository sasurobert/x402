package multiversx

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"github.com/coinbase/x402/go/mechanisms/multiversx"
	"github.com/multiversx/mx-sdk-go/data"
)

// ClientSigner implements multiversx.ClientMultiversXSigner using local Ed25519 keys
type ClientSigner struct {
	privKey ed25519.PrivateKey
	address string
}

// NewClientSignerFromPrivateKey creates a new ClientSigner from a hex-encoded private key (seed)
func NewClientSignerFromPrivateKey(privKeyHex string) (*ClientSigner, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key hex: %w", err)
	}

	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privKeyBytes))
	}

	privKey := ed25519.NewKeyFromSeed(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	address, err := data.NewAddressFromBytes(pubKey).AddressAsBech32String()
	if err != nil {
		return nil, fmt.Errorf("failed to derive bech32 address: %w", err)
	}

	return &ClientSigner{
		privKey: privKey,
		address: address,
	}, nil
}

// Ensure ClientSigner implements ClientMultiversXSigner interface
var _ multiversx.ClientMultiversXSigner = (*ClientSigner)(nil)

// Address returns the bech32 address of the signer
func (s *ClientSigner) Address() string {
	return s.address
}

// Sign signs the message bytes and returns the signature
func (s *ClientSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.privKey, message), nil
}

// PrivateKey returns the private key bytes of the signer
func (s *ClientSigner) PrivateKey() []byte {
	return s.privKey
}
