package multiversx

import (
	"context"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/data"
)

// ClientMultiversXSigner defines the interface for signing MultiversX transactions
type ClientMultiversXSigner interface {
	// Address returns the bech32 address of the signer
	Address() string

	// Sign signs the transaction object/bytes and returns the signature hex
	// In strict MultiversX terms, we sign the canonical JSON of the transaction fields.
	// For this interface, we pass the bytes to be signed.
	Sign(ctx context.Context, message []byte) ([]byte, error)

	// PrivateKey returns the private key bytes of the signer
	PrivateKey() []byte
}

// FacilitatorMultiversXSigner defines the interface for facilitator MultiversX operations
type FacilitatorMultiversXSigner interface {
	// GetAddresses returns all addresses this facilitator can use for signing
	GetAddresses() []string

	// Sign signs the transaction and returns the signature as a hex string
	Sign(ctx context.Context, tx *transaction.FrontendTransaction) (string, error)

	// SendTransaction sends a transaction to the network
	SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error)

	// GetAccount fetches account details (nonce, balance)
	GetAccount(ctx context.Context, address string) (*data.Account, error)

	// GetTransactionStatus fetches the status of a transaction
	GetTransactionStatus(ctx context.Context, txHash string) (string, error)
}
