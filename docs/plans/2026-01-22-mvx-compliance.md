# MultiversX Compliance Improvement Plan

**Goal:** Refine the MultiversX integration to strictly adhere to the `go/CONTRIBUTING.md` standards and address code review findings.

**Architecture:** 
1. **Error Handling Upgrade:** Replace generic `fmt.Errorf` with `x402.NewVerificationError` or `x402.NewConfigurationError` across the MultiversX mechanism.
2. **Documentation Update:** Expand `go/mechanisms/multiversx/README.md` to document new features (Local Verification, Gas Rules, Token Validation).
3. **Test Compliance:** Move/Copy integration tests to `go/test/integration/multiversx_integration_test.go` to satisfy the central testing requirement while preserving module autonomy.

**Tech Stack:** Go 1.24+

---

### Task 1: Refactor Error Handling (Server)

**Files:**
- Modify: `go/mechanisms/multiversx/exact/server/scheme.go`

**Step 1: Write the failing test**
Create `go/mechanisms/multiversx/exact/server/error_test.go`:
```go
package server

import (
    "testing"
    x402 "github.com/coinbase/x402/go"
)

func TestValidatePaymentRequirements_ReturnsTypedError(t *testing.T) {
    scheme := NewExactMultiversXScheme()
    req := x402.PaymentRequirements{
        PayTo: "invalid-address",
        Amount: "100",
        Asset: "EGLD",
    }
    
    err := scheme.ValidatePaymentRequirements(req)
    if err == nil {
        t.Fatal("Expected error")
    }
    
    // Check if error is of type x402.VerificationError (or ConfigurationError depending on semantic)
    // ValidatePaymentRequirements usually implies client sent bad data -> VerificationError is likely appropriate or just standard error? 
    // Actually CONTRIBUTING says: `x402.NewVerificationError`. 
    // Let's check type assertion.
    // Note: If x402.VerificationError isn't exported as a struct we check standard unwrapping or interface.
    // Assuming x402 exposes IsVerificationError or similar? Or just check if we can cast it.
    // The snippet showed: return nil, x402.NewVerificationError(...)
}
```
*Note: I need to verify `x402` error types available first. Assuming `NewVerificationError` is the target.*

**Step 2: Run test to verify it fails**
Run: `go test -v ./mechanisms/multiversx/exact/server/error_test.go`
Expected: FAIL (because current impl returns `fmt.Errorf`)

**Step 3: Modify implementation**
In `scheme.go`:
- Replace `fmt.Errorf` with `x402.NewVerificationError` (or appropriate) for validation failures.

**Step 4: Run test to verify it passes**
Run: `go test -v ./mechanisms/multiversx/exact/server/error_test.go`
Expected: PASS

**Step 5: Commit**
```bash
git add go/mechanisms/multiversx/exact/server/scheme.go go/mechanisms/multiversx/exact/server/error_test.go
git commit -m "fix(mvx): use typed errors in server scheme validation"
```

### Task 2: Refactor Error Handling (Facilitator)

**Files:**
- Modify: `go/mechanisms/multiversx/exact/facilitator/scheme.go`
- Modify: `go/mechanisms/multiversx/verify.go`

**Step 1: Write the failing test**
Update `go/mechanisms/multiversx/verify_test.go` to check error type.

**Step 2: Run test to verify it fails**
Run: `go test -v ./mechanisms/multiversx/verify_test.go`

**Step 3: Modify implementation**
- In `verify.go`: Return `x402.NewVerificationError` for signature failures.
- In `facilitator/scheme.go`: Wrap errors `x402.NewVerificationError` during Verify flow.

**Step 4: Run test to verify it passes**
Run: `go test -v ./mechanisms/multiversx/verify_test.go`

**Step 5: Commit**
```bash
git add go/mechanisms/multiversx/verify.go go/mechanisms/multiversx/exact/facilitator/scheme.go
git commit -m "fix(mvx): use typed errors in facilitator and verify logic"
```

### Task 3: Global Integration Test

**Files:**
- Create: `go/test/integration/multiversx_test.go`

**Step 1: Create the test file**
Copy content from `go/mechanisms/multiversx/tests/integration_test.go` to `go/test/integration/multiversx_test.go`, adjusting package name to `integration_test` and imports.

**Step 2: Run test to verify it passes**
Run: `go test -v ./test/integration/multiversx_test.go`
Expected: PASS (It should partial pass or skip if no devnet, but mock tests should pass)

**Step 3: Commit**
```bash
git add go/test/integration/multiversx_test.go
git commit -m "test(mvx): add global integration tests"
```

### Task 4: Documentation Update

**Files:**
- Modify: `go/mechanisms/multiversx/README.md`

**Step 1: Write content**
Add sections for:
- **Local Verification**: Explain the Ed25519 check.
- **Gas Calculation**: Document the formula.
- **Token Validation**: Explain strict rules.

**Step 2: Verify rendering**
(Manual check)

**Step 3: Commit**
```bash
git add go/mechanisms/multiversx/README.md
git commit -m "docs(mvx): update readme with parity features"
```
