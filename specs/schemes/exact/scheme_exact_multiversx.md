# Scheme: `exact` on `MultiversX`

## Summary

The `exact` scheme on MultiversX facilitates payments where the **Client** (user) signs a transaction payload, and the **Facilitator** (relayer) broadcasts it, potentially paying the gas fees ("Relayed Protocol").
This enables a gasless experience for the user.

This is implemented via two methods depending on the token type:

| AssetTransferMethod | Use Case                                      | Details                                                                 |
| :------------------ | :-------------------------------------------- | :---------------------------------------------------------------------- |
| **1. Direct**       | Native EGLD transfers.                        | Uses a basic transaction. If paying a Smart Contract, the data field calls the purchase function (e.g., `pay` or `buy`) and may include a resource ID. For user transfers, the data field is optional. |
| **2. ESDT**         | Token transfers (Fungible/SFT/Meta).          | Uses `MultiESDTNFTTransfer` built-in function to send tokens + trigger. |

MultiversX transactions natively support a specific structure (Sender, Receiver, Value, Data, etc.). The "Relayed" v3 architecture allows the Client to sign this structure, and the Facilitator to wrap it in a transaction that pays the gas (via Protocol Relayed Transactions). Note: The `exact` scheme name refers to the exact payment amount required. However, the x402 standard generally suggests the Facilitator covers gas costs.

**Note on Relayed V3 (Protocol Level):**
MultiversX supports "Relayed Transactions" where a relayer submits a transaction signed by a sender `A`. The relayer pays for gas. We use this mechanism.


Both methods can trigger complex Smart Contract calls.

## Use Cases & Flows

| Case | Method | Description |
| :--- | :--- | :--- |
| **Simple Pay** | Direct | User signs a tx to send EGLD to a Merchant. |
| **Token Pay** | ESDT | User signs a tx to send USDC/Tokens to a Merchant. |
| **SC Call** | Direct/ESDT | User signs a tx to call a SC function (e.g. `buyItem`) with arguments. |

---

## 1. AssetTransferMethod: `Direct` (EGLD)

Used for native EGLD transfers or direct Smart Contract calls.

### A. Simple Transfer
Sending EGLD from Client to Merchant.

## B. Smart Contract Call (Complex)
Calling a function `buy` with arguments `[0x01, 0xABC]` while sending EGLD.

**Payment Header:**
- `extra`: 
  ```json
  {
    "assetTransferMethod": "direct",
    "scFunction": "buy",
    "arguments": ["01", "abc"]
  }
  ```

**Payload Data:** `buy@01@abc`

**Client Payload Structure (JSON):**

```json
{
  "x402Version": 2,
  "resource": {
    "url": "https://api.merchant.com/v1/buy",
    "description": "Purchase of item #123",
    "mimeType": "application/json"
  },
  "accepted": {
    "scheme": "exact",
    "network": "multiversx:1",
    "amount": "1000000000000000000",
    "asset": "EGLD",
    "payTo": "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
    "maxTimeoutSeconds": 60,
    "extra": {
      "assetTransferMethod": "direct",
      "scFunction": "buy",
      "arguments": ["01"]
    }
  },
  "payload": {
    "nonce": 15,
    "value": "1000000000000000000",
    "receiver": "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
    "sender": "erd1client...",
    "gasPrice": 1000000000,
    "gasLimit": 50000,
    "data": "buy@01",
    "chainID": "1",
    "version": 1,
    "signature": "aaff...",
    "validAfter": 1740672089,
    "validBefore": 1740672154
  }
}
```

### Phase 2: Verification Logic

1.  **Verify Signature**: Ensure the Ed25519 signature is valid and recoveries to the `sender` address.
2.  **Verify Balance**: High-level check of the `sender`'s native balance.
3.  **Verify Amount & Destination**: Ensure `value` matches requirement `amount` and `receiver` matches `payTo`.
4.  **Verify Logic**: If `scFunction` is present, ensure `data` begins with the correct function name and contains valid arguments hex-encoded. Otherwise, the `data` field check is optional.
5.  **Simulation**: Use `POST /transaction/simulate` to ensure the transaction would succeed with the given state.


---

## 2. AssetTransferMethod: `ESDT` (Tokens)

Used for any standard token (Fungible, SFT, NFT) on MultiversX.

### Data Structure

The payload for ESDT is significantly different to accommodate the `MultiESDTNFTTransfer` call. Note that for this method, the `receiver` of the transaction is often the **Sender (Self)** to trigger the built-in transfer mechanism.

Format: `MultiESDTNFTTransfer @ <Receiver_Hex> @ <Num_Tokens_Hex> @ <Token_ID_Hex> @ <Nonce_Hex> @ <Amount_Hex> [@ <scFunction_Hex> [@ <Args_Hex>...]]`


### Smart Contract Call with Tokens
Calling `deposit` on a contract while sending USDC.

**Payment Header:**
- `extra`:
  ```json
  {
    "assetTransferMethod": "esdt",
    "scFunction": "deposit",
    "arguments": ["05"] // e.g. lock period
  }
  ```

**Payload Data:**
- Format: `MultiESDTNFTTransfer @ <ContractHex> @ 01 @ <TokenHex> @ 00 @ <AmountHex> @ <FunctionHex> @ <ArgsHex>...`
- Example: `MultiESDTNFTTransfer@...&deposit@05`

---
### Phase 1: `PAYMENT-SIGNATURE` Header Payload

**Example PaymentPayload:**

```json
{
  "x402Version": 2,
  "resource": {
    "url": "https://api.example.com/premium-data",
    "description": "Token-based purchase",
    "mimeType": "application/json"
  },
  "accepted": {
    "scheme": "exact",
    "network": "multiversx:D",
    "amount": "1000000",
    "asset": "USDC-c70f1a",
    "payTo": "erd1merchant...",
    "extra": {
      "assetTransferMethod": "esdt",
      "scFunction": "deposit",
      "arguments": ["05"]
    }
  },
  "payload": {
    "nonce": 16,
    "value": "0",
    "receiver": "erd1client...",
    "sender": "erd1client...",
    "gasPrice": 1000000000,
    "gasLimit": 60000000,
    "data": "MultiESDTNFTTransfer@<merchant_hex>@01@<usdc_hex>@00@<amount_hex>@6465706f736974@05",
    "chainID": "D",
    "version": 1,
    "signature": "cc55...",
    "validAfter": 1740672089,
    "validBefore": 1740672154
  }
}
```

### Phase 2: Verification Logic

1.  **Verify Signature**: Ensure valid Ed25519 signature.
2.  **Parse Data Field**:
    - Ensure it starts with `MultiESDTNFTTransfer`.
    - Extract Destination, Token Identifier, Amount, and optional `scFunction`.
3.  **Verify Extracted Data**:
    - Destination (hex decoded) must match `payTo`.
    - Token Identifier must match `asset`.
    - Amount (hex decoded) must meet or exceed the required `amount`.
    - `scFunction` (if required) must match the expected method.
4.  **Simulation**: Run simulation via proxy API to catch storage or programmatic failures.

---

## 3. Relayed Execution (Protocol V3)

MultiversX natively supports **Relayed Transactions** where a relayer submits a transaction signed by a sender `A`. The relayer pays for gas. We use this mechanism to enable gasless payments.

### Execution Flow

1.  **Client** creates the `Transaction` payload.
2.  **Client** signs the `Transaction` (canonical JSON).
3.  **Client** sends `ExactRelayedPayload` to the Facilitator.
4.  **Facilitator** wraps the Client's payload in a **Relayed Transaction V3** (Protocol Native).
    - Facilitator submits the tx directly to the network's relayed endpoint.
    - The inner transaction is executed and gas is paid by the Facilitator.
5.  **Facilitator** broadcasts.

*Implementation Note:* The Facilitator constructs the Relayed V3 transaction:
- Inner Transaction: The signed payload from the client.
- Relayer: The Facilitator's address (payer of gas).

### Security & Simulation Note
The Facilitator MUST simulate the transaction before broadcasting (using `POST /transaction/simulate`) to ensure:
1.  **Payment Verification**: The transaction pays the required amount to the merchant.
2.  **Solvency**: The Facilitator has enough funds to cover the gas for the relayed wrapper.
3.  **Correctness**: The execution doesn't revert (which would still consume relayer gas).
4.  **Security**: The transaction doesn't maliciously exploit the relayer (though protocol protections exist).
