# Scheme: `exact` on `MultiversX`

## Summary

The `exact` scheme on MultiversX facilitates payments where the **Client** (user) signs a transaction payload, and the **Facilitator** (relayer) broadcasts it, potentially paying the gas fees ("Relayed Protocol").

It supports two main transfer methods:
1.  **Direct (EGLD)**: Native currency transfer or smart contract call.
2.  **ESDT (Tokens)**: Token transfer via `MultiESDTNFTTransfer`.

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

**Payment Header:**
- `asset`: "EGLD"
- `payTo`: Merchant Address
- `amount`: "1000000..."
- `extra`: `{ "assetTransferMethod": "direct" }`

**Payload Data:** `""` (empty)

### B. Smart Contract Call (Complex)
Calling a function `buy` with arguments `[0x01, 0xABC]` while sending EGLD.

**Payment Header:**
- `extra`: 
  ```json
  {
    "assetTransferMethod": "direct",
    "resourceId": "buy",
    "arguments": ["01", "abc"]
  }
  ```

**Payload Data:** `buy@01@abc`

---

## 2. AssetTransferMethod: `ESDT` (Tokens)

Used for any standard token (Fungible, SFT, NFT).

### A. Simple Transfer
Sending USDC to Merchant.

**Payment Header:**
- `asset`: "USDC-123456"
- `payTo`: Merchant Address
- `amount`: "1000000"

**Payload Data:** 
- Constructed via `MultiESDTNFTTransfer`.
- Format: `MultiESDTNFTTransfer @ <MerchantHex> @ 01 @ <TokenHex> @ 00 @ <AmountHex>`

### B. Smart Contract Call with Tokens
Calling `deposit` on a contract while sending USDC.

**Payment Header:**
- `extra`:
  ```json
  {
    "assetTransferMethod": "esdt",
    "resourceId": "deposit",
    "arguments": ["05"] // e.g. lock period
  }
  ```

**Payload Data:**
- Format: `MultiESDTNFTTransfer @ <ContractHex> @ 01 @ <TokenHex> @ 00 @ <AmountHex> @ <FunctionHex> @ <ArgsHex>...`
- Example: `MultiESDTNFTTransfer@...&deposit@05`

---

## 3. Relayed Execution (Protocol V3)

Use `assetTransferMethod` = `relayed` (or implicit by Facilitator logic) to indicating gas sponsorship.

**Flow:**
1.  **Client** creates the `Transaction`.
2.  **Client** signs the `Transaction` (canonical JSON).
3.  **Client** sends `ExactRelayedPayload` to Facilitator.
4.  **Facilitator** wraps it in a Protocol Relayed V3 Transaction (paying gas).
5.  **Facilitator** broadcasts.

**Client Payload Structure (JSON):**

```json
{
  "nonce": 10,
  "value": "1000",
  "receiver": "erd1...",
  "sender": "erd1...",
  "gasPrice": 1000000000,
  "gasLimit": 50000,
  "data": "buy@01",
  "chainID": "1",
  "version": 1,
  "signature": "aaff..." 
}
```

### Verification Logic

1.  **Signature**: Verify Ed25519 signature of the Client's sender address against the inner transaction.
2.  **Simulation**: Run `simulate` on the inner transaction (or the relayed wrapper) to ensure success.
3.  **Constraints**: Check `payTo` matches Receiver (or Data destination for ESDT), `amount` matches Value (or Data amount for ESDT).
