package multiversx

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	crypto "github.com/multiversx/mx-chain-crypto-go"
	"github.com/multiversx/mx-chain-crypto-go/signing"
	mxed25519 "github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	"github.com/multiversx/mx-sdk-go/builders"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"
)

// SimpleCryptoHolder implements core.CryptoComponentsHolder
type SimpleCryptoHolder struct {
	privateKey crypto.PrivateKey
	publicKey  crypto.PublicKey
	address    core.AddressHandler
}

// NewSimpleCryptoHolder creates a new crypto holder from a private key hex string
func NewSimpleCryptoHolder(privateKeyHex string) (*SimpleCryptoHolder, error) {
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	return NewSimpleCryptoHolderFromBytes(privKeyBytes)
}

// NewSimpleCryptoHolderFromBytes creates a new crypto holder from private key bytes
func NewSimpleCryptoHolderFromBytes(privKeyBytes []byte) (*SimpleCryptoHolder, error) {
	suite := mxed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(suite)

	privKey, err := keyGen.PrivateKeyFromByteArray(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}
	pubKey := privKey.GeneratePublic()

	pubKeyBytes, err := pubKey.ToByteArray()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key bytes: %w", err)
	}

	address := data.NewAddressFromBytes(pubKeyBytes)

	return &SimpleCryptoHolder{
		privateKey: privKey,
		publicKey:  pubKey,
		address:    address,
	}, nil
}

func (h *SimpleCryptoHolder) GetPublicKey() crypto.PublicKey {
	return h.publicKey
}

func (h *SimpleCryptoHolder) GetPrivateKey() crypto.PrivateKey {
	return h.privateKey
}

func (h *SimpleCryptoHolder) GetBech32() string {
	val, _ := h.address.AddressAsBech32String()
	return val
}

func (h *SimpleCryptoHolder) GetAddressHandler() core.AddressHandler {
	return h.address
}

func (h *SimpleCryptoHolder) IsInterfaceNil() bool {
	return h == nil
}

// SimpleSigner implements builders.Signer
type SimpleSigner struct{}

func (s *SimpleSigner) SignMessage(msg []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	return s.SignByteSlice(msg, privateKey)
}

func (s *SimpleSigner) VerifyMessage(msg []byte, publicKey crypto.PublicKey, sig []byte) error {
	return fmt.Errorf("VerifyMessage not implemented")
}

func (s *SimpleSigner) SignTransaction(tx *transaction.FrontendTransaction, privateKey crypto.PrivateKey) ([]byte, error) {
	return nil, fmt.Errorf("SignTransaction not implemented (use builder)")
}

func (s *SimpleSigner) SignByteSlice(msg []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	// Extract raw ed25519 private key
	scalar := privateKey.Scalar()
	underlying := scalar.GetUnderlyingObj()
	privKeyBytes, ok := underlying.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("invalid private key type")
	}
	return ed25519.Sign(privKeyBytes, msg), nil
}

func (s *SimpleSigner) VerifyByteSlice(msg []byte, publicKey crypto.PublicKey, sig []byte) error {
	return fmt.Errorf("VerifyByteSlice not implemented")
}

func (s *SimpleSigner) IsInterfaceNil() bool {
	return s == nil
}

// SignTransactionWithBuilder signs a transaction using the SDK builder
// If asRelayer is true, it applies the relayer signature. Otherwise, it applies the user signature.
func SignTransactionWithBuilder(holder core.CryptoComponentsHolder, tx *transaction.FrontendTransaction, asRelayer bool) error {
	builder, err := builders.NewTxBuilder(&SimpleSigner{})
	if err != nil {
		return fmt.Errorf("failed to create tx builder: %w", err)
	}

	if asRelayer {
		return builder.ApplyRelayerSignature(holder, tx)
	}
	return builder.ApplyUserSignature(holder, tx)
}
