# MultiversX Mechanism (V2)

This package implements x402 payment support for the MultiversX network.
It follows the standard x402 V2 architecture with support for the "Relayed" (Gasless) model.

## Subpackages

- `exact/server`: Server-side logic for parsing prices and checking requirements.
- `exact/facilitator`: Facilitator logic for verifying signatures and simulating transactions on-chain.
- `exact/client`: Client-side logic (Stub).

## Features

### 1. Robust Verification Strategy
The facilitator implements a hybrid verification approach for maximum security and performance:
1.  **Local Verification**: It first attempts to verify the Ed25519 signature locally against the sender's public key (derived from Bech32 address). This avoids unnecessary network calls for invalid signatures.
2.  **Simulation Fallback**: If local verification passes (or cannot be performed), it submits the transaction to the MultiversX Gateway `simulation` endpoint to ensure protocol validity (nonce, balance, rules).

### 2. Gas Calculation
Gas is calculated automatically based on the protocol formula:
```
GasLimit = 50,000 (Base) 
         + 1,500 * DataBytes 
         + 200,000 * NumTransfers 
         + 50,000 (Relayed)
```

### 3. Strict Validation
- **TokenID**: Validates ESDT identifiers against regex `^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$`.
- **Address**: Validates proper Bech32 HRP (`erd`) and checksum.
- **Amounts**: Ensures high-precision formatting using `big.Int`.

## Usage

### Server (Merchant)

```go
import (
    "github.com/coinbase/x402/go/mechanisms/multiversx/exact/server"
    "github.com/coinbase/x402/go/mechanisms/multiversx/exact/facilitator"
)

// 1. Setup Support
scheme := server.NewExactMultiversXScheme()

// 2. Setup Facilitator (for verification)
verifier := facilitator.NewExactMultiversXScheme("https://devnet-gateway.multiversx.com")

// 3. Verify Payment
// Returns x402.VerifyError on failure
simHash, err := verifier.Verify(ctx, payload, requirements)
```
