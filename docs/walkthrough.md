# MultiversX Parity Walkthrough

**Goal**: Achieve feature parity with EVM implementation for MultiversX integration in `go/mechanisms/multiversx`.

## 1. Validation Utilities (`utils.go`)
Added strict validation for addresses and ESDT TokenIDs.

```go
// IsValidTokenID checks via Regex ^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$
func IsValidTokenID(tokenID string) bool { ... }

// IsValidAddress verifies Bech32 checksum and HRP "erd"
func IsValidAddress(address string) bool { ... }
```
**Tests (`utils_test.go`)**: Covered valid/invalid addresses, token ticker lengths, and nonce hex validity.

## 2. Gas Calculation (`utils.go`)
Implemented precise gas estimation matching protocol formula.

**Formula**: `50,000 (Base) + 1,500 * DataBytes + 200,000 * NumTransfers + 50,000 (Relayed)`

```go
func CalculateGasLimit(data []byte, numTransfers int) uint64 { ... }
```
**Tests**: Verified base cases, data scaling, and multi-transfer additions.

## 3. Robust Scheme Parsing (`server/scheme.go`)
Enhanced `ExactMultiversXScheme` to handle various input formats reliably.

*   **Money Parsing**: handles `string`, `float64`, `int`, and passed-through `map`.
*   **Validation**: `ValidatePaymentRequirements` strictly checks `PayTo` (Bech32), `Amount` (BigInt), and `Asset` (TokenID).
*   **Enhancement**: Defaults asset to `EGLD` if missing.

## 4. Local Verification (`verify.go`)
Implemented hybrid verification strategy for maximum security and performance.

1.  **Local Ed25519 Check**: Verifies the signature against the `sender` address (pubkey) locally using `crypto/ed25519`. FAST & SECURE.
2.  **Simulation Fallback**: If local check fails (rare, maybe multisig), falls back to multiversx-api simulation.

```go
// VerifyPayment checks signature locally first, then simulates if needed.
func VerifyPayment(...) (bool, error) { ... }
```

**Tests (`verify_test.go`)**: 
*   Generated real Ed25519 keypairs.
*   Signed canonical JSON payloads.
*   Verified successful validation without hitting the mock simulator.
*   Verified failure on tamper.

## 5. Integration Tests (`integration_test.go`)
Updated integration tests to use valid Bech32 addresses and valid hex signatures to survive strict validation.

## Verification Result
All tests passed successfully.

```
ok      github.com/coinbase/x402/go/mechanisms/multiversx       0.308s
ok      github.com/coinbase/x402/go/mechanisms/multiversx/exact/client  (cached)
ok      github.com/coinbase/x402/go/mechanisms/multiversx/exact/server  0.181s
ok      github.com/coinbase/x402/go/mechanisms/multiversx/tests 0.662s
```
