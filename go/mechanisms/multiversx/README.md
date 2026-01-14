# MultiversX Mechanism (Go)

This package implements the **x402** verification logic for the MultiversX network.

## Architecture: Relayed Payment Model (V3)

The verified payment flow uses a "Relayed" architecture to enable gasless transactions for the end-user (payer).

1.  **Client (Payer)**:
    -   Constructs a MultiversX transaction (EGLD transfer or ESDT `MultiESDTNFTTransfer`).
    -   Signs the transaction.
    -   Sends the **Signature** and **Transaction Payload** (nonce, values, data, etc.) to the Facilitator/Server.
    -   *Does NOT broadcast the transaction to the network directly.*

2.  **Server (Verifier)**:
    -   Receives the `RelayedPayload`.
    -   **Verifies** the validity of the transaction using **Transaction Simulation**.
    -   Validates business logic (Receiver address, Payment Amount, Resource ID).
    -   (If valid) Broadcasts the transaction to the network via a Relayer (or the Merchant's wallet).

## Verification Logic

The `Verifier` performs the following checks:

### 1. Transaction Simulation (Security)
To ensure the signature is valid and the transaction is executable (correct nonce, sufficient balance), the Verifier performs a **Simulation** against the MultiversX Network.

-   **Endpoint**: POST `<API_URL>/transaction/simulate`
-   **Payload**: The exact signed transaction fields received from the client.
-   **Check**: The simulation response status must be `success`.

This approach avoids implementing complex offline cryptographic verification (Bech32 decoding, canonical JSON serialization) within the Go service, deferring validity checks to the authoritative network node.

### 2. Business Logic Validation
After the simulation confirms specific integrity, the Verifier checks:
-   **Receiver**: Matches the expected merchant/contract address.
    -   *Note*: For ESDT transfers, the `Receiver` field in the tx is the Sender (self-transfer), so the `Data` field is parsed to verify the destination.
-   **Amount**: Matches the required payment amount.
-   **Resource ID**: Confirms the payment is for the specific invoice/resource (checked against the `Data` field).

## Usage

```go
import "github.com/coinbase/x402/go/mechanisms/multiversx"

// Initialize with a Proxy API URL (e.g., https://devnet-gateway.multiversx.com)
verifier := multiversx.NewVerifier("https://devnet-gateway.multiversx.com")

// Process a payment
txHash, err := verifier.ProcessRelayedPayment(
    payload,           // RelayedPayload from client
    expectedReceiver,  // Your wallet address
    resourceId,        // Expected Invoice ID
    expectedAmount,    // Expected Value
    tokenIdentifier,   // "EGLD" or Token ID
)
```
