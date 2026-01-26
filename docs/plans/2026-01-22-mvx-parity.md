# MultiversX Parity Implementation Plan

**Goal:** Bring the MultiversX Go implementation to feature parity with EVM, adding robust money parsing, strict input validation, gas verification, and local signature verification.

**Architecture:** 
- Enhance `scheme.go` with parsing logic and validation hooks.
- Create `utils.go` (or expand it) with `IsValidAddress`, `IsValidTokenID`, and `CalculateGas`.
- Implement local Ed25519 verification in `verify.go` before falling back to simulation.
- Ensure >80% test coverage with table-driven tests and no mocking of core logic.

**Tech Stack:** Go, standard library, `github.com/coinbase/x402/go` types.

---

### Task 1: Validation Utilities (Address & TokenID)

**Files:**
- Modify: `go/mechanisms/multiversx/utils.go` (Ensure absolute path in tools)
- Test: `go/mechanisms/multiversx/utils_test.go`

**Step 1: Write failing tests for Validation**
Create `utils_test.go` (if not robust) and add `TestIsValidTokenID` and `TestIsValidAddress`.
```go
// go/mechanisms/multiversx/utils_test.go
package multiversx

import (
    "testing"
)

func TestIsValidTokenID(t *testing.T) {
    tests := []struct {
        name    string
        tokenID string
        valid   bool
    }{
        {"EGLD", "EGLD", false}, 
        {"Valid USDC", "USDC-123456", true},
        {"Valid WEGLD", "WEGLD-abcdef", true},
        {"Too Short Ticker", "A-123456", false}, 
        {"Too Long Ticker", "TOOLONGNA-123456", false},
        {"Invalid Nonce Length", "USDC-1234567", false},
        {"Invalid Nonce Char", "USDC-12345G", false},
        {"No Dash", "USDC123456", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsValidTokenID(tt.tokenID); got != tt.valid {
                t.Errorf("IsValidTokenID(%q) = %v; want %v", tt.tokenID, got, tt.valid)
            }
        })
    }
}

func TestIsValidAddress(t *testing.T) {
    // Implement table driven tests for address
}
```

**Step 2: Run tests to verify it fails**
`go test ./go/mechanisms/multiversx/...`

**Step 3: Write minimal implementation**
In `go/mechanisms/multiversx/utils.go`:
```go
package multiversx

import (
    "regexp"
)

var tokenIDRegex = regexp.MustCompile(`^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$`)

func IsValidTokenID(tokenID string) bool {
    return tokenIDRegex.MatchString(tokenID)
}

func IsValidAddress(address string) bool {
    // Implement check
    return true // placeholder
}
```

**Step 4: Run test to verify it passes**
`go test ./go/mechanisms/multiversx/...`

**Step 5: Commit**

---

### Task 2: Gas Calculation Logic

**Files:**
- Modify: `go/mechanisms/multiversx/utils.go`
- Test: `go/mechanisms/multiversx/utils_test.go`

**Step 1: Write failing tests**
```go
func TestCalculateGasLimit(t *testing.T) {
    // Tests
}
```

**Step 2: Run tests to fail**

**Step 3: Implement Logic**
```go
func CalculateGasLimit(data []byte, numTransfers int) uint64 {
    // Formula: 
    // Base Cost: 50,000
    // Data Cost: 1,500 * len(data)
    // MultiTransfer Cost: 200,000 * numTransfers
    // Relayed Cost: 50,000
    
    // gas = 50000 + 1500*len(data) + 200000*numTransfers + 50000
    const BaseCost = 50000
    const GasPerByte = 1500
    const MultiTransferCost = 200000
    const RelayedCost = 50000

    return BaseCost + 
           (GasPerByte * uint64(len(data))) + 
           (MultiTransferCost * uint64(numTransfers)) + 
           RelayedCost
}
```

**Step 4: Run tests to pass**

**Step 5: Commit**

---

### Task 3: Robust Money Parsing (Scheme)

**Files:**
- Modify: `go/mechanisms/multiversx/exact/server/scheme.go`
- Test: `go/mechanisms/multiversx/exact/server/scheme_test.go`

**Step 1: Write failing tests**
Test `ParsePrice` with strings, floats, maps, and invalid inputs.

**Step 2: Run tests**

**Step 3: Implement Logic**
- Add `moneyParsers` field to struct.
- Implement `ParsePrice` to handle types and use `parseMoneyToDecimal`.

**Step 4: Run tests**

**Step 5: Commit**

---

### Task 4: Scheme Validation Hardening

**Files:**
- Modify: `go/mechanisms/multiversx/exact/server/scheme.go`
- Test: `go/mechanisms/multiversx/exact/server/scheme_test.go`

**Step 1: Update tests**
Add invalid address/tokenID cases to `ValidatePaymentRequirements`.

**Step 2: Implement validation**

**Step 3: Verify pass**

**Step 4: Commit**

---

### Task 5: Local Verification

**Files:**
- Modify: `go/mechanisms/multiversx/verify.go`
- Test: `go/mechanisms/multiversx/verify_test.go`

**Step 1: Write failing tests**

**Step 2: Implement Local Verification**
- Reconstruct tx bytes.
- `ed25519.Verify`.

**Step 3: Pass tests**

**Step 4: Commit**
