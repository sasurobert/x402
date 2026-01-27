# MultiversX Client Signer

Client-side Ed25519 signing for MultiversX-based x402 payments.

## Usage

```go
import (
    mxclient "github.com/coinbase/x402/go/mechanisms/multiversx/exact/client"
    mxsigners "github.com/coinbase/x402/go/signers/multiversx"
)

// Create signer from private key (hex encoded or seed)
signer, err := mxsigners.NewClientSignerFromPrivateKey("413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9")
if err != nil {
    log.Fatal(err)
}

// Use with ExactMultiversXScheme
mxScheme, err := mxclient.NewExactMultiversXScheme(signer, "multiversx:D")
```

## API

### NewClientSignerFromPrivateKey

```go
func NewClientSignerFromPrivateKey(privKeyHex string) (*ClientSigner, error)
```

Creates a client signer from a hex-encoded private key (seed).

**Args:**
- `privKeyHex`: Hex-encoded private key (32 bytes)

**Returns:**
- `*ClientSigner` implementation
- Error if key is invalid

**Examples:**

```go
// From hex string
signer, _ := mxsigners.NewClientSignerFromPrivateKey("413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9")

// From environment variable
signer, _ := mxsigners.NewClientSignerFromPrivateKey(os.Getenv("MX_PRIVATE_KEY"))
```

## Interface Implementation

The helper implements `multiversx.ClientMultiversXSigner`:

```go
type ClientMultiversXSigner interface {
    Address() string
    Sign(ctx context.Context, message []byte) ([]byte, error)
    PrivateKey() []byte
}
```

### Methods

**`Address() string`**
- Returns the Bech32 address of the signer
- Example: `"erd1..."`

**`Sign(ctx context.Context, message []byte) ([]byte, error)`**
- Signs arbitrary messages (or transaction bytes) using Ed25519
- Returns 64-byte signature

**`PrivateKey() []byte`**
- Returns the raw 32-byte private key (seed)
- Used for internal SDK interoperability

## Supported Networks

Works with all MultiversX network types (configured via scheme):

- **Devnet**: `multiversx:D`
- **Testnet**: `multiversx:T`
- **Mainnet**: `multiversx:1`

## Dependencies

- `github.com/multiversx/mx-sdk-go` - Official MultiversX Go SDK
- `github.com/coinbase/x402/go/mechanisms/multiversx` - x402 MultiversX mechanism types

## Testing

Run tests:

```bash
go test github.com/coinbase/x402/go/signers/multiversx -v
```
