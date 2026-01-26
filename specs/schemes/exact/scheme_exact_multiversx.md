# Scheme: `exact` on `MultiversX`

## Summary

The `exact` scheme on MultiversX executes a transfer where the Facilitator (server/relayer) pays the gas, while the Client (user) signs the transaction payload. This enables a gasless experience for the user.

This is implemented via two methods depending on the token type:

| AssetTransferMethod | Use Case                                      | Details                                                                 |
| :------------------ | :-------------------------------------------- | :---------------------------------------------------------------------- |
| **1. Direct**       | Native EGLD transfers.                        | Uses a basic transaction. If paying a Smart Contract, the data field calls the purchase function (e.g., `pay` or `buy`) and may include a resource ID. For user transfers, the data field is optional. |
| **2. ESDT**         | Token transfers (Fungible/SFT/Meta).          | Uses `MultiESDTNFTTransfer` built-in function to send tokens + trigger. |

MultiversX transactions natively support a specific structure (Sender, Receiver, Value, Data, etc.). The "Relayed" v3 architecture allows the Client to sign this structure, and the Facilitator to wrap it in a transaction that pays the gas (via Protocol Relayed Transactions). Note: The `exact` scheme name refers to the exact payment amount required. However, the x402 standard generally suggests the Facilitator covers gas costs.

**Note on Relayed V3 (Protocol Level):**
MultiversX supports "Relayed Transactions" where a relayer submits a transaction signed by a sender `A`. The relayer pays for gas. We use this mechanism.

---

## 1. AssetTransferMethod: `Direct` (EGLD)

Used for native currency transfers.

### Phase 1: `PAYMENT-SIGNATURE` Header Payload

The `payload` field must contain the signed transaction components.

**Example PaymentPayload:**

```json
{
  "x402Version": 2,
  "resource": {
    "url": "https://api.example.com/premium-data",
    "description": "Premium Market Data Feed",
    "mimeType": "application/json"
  },
  "accepted": {
    "network": "multiversx:D", // "D" for Devnet, "1" for Mainnet, "T" for Testnet
    "amount": "1000000000000000000", // 1 EGLD
    "asset": "EGLD",
    "payTo": "erd1qqqqqqqq...", // Smart Contract or Merchant Address
    "extra": {
      "assetTransferMethod": "direct"
    }
  },
  "payload": {
    "nonce": 15,
    "value": "1000000000000000000",
    "receiver": "erd1qqqqqqqq...",
    "sender": "erd1client...",
    "gasPrice": 1000000000,
    "gasLimit": 50000,
    "data": "pay@<resource_id_hex>",
    "chainID": "D",
    "version": 1,
    "validAfter": 1706200000,
    "validBefore": 1706200600
  }
}
```

### Phase 2: Verification Logic

1.  **Verify** the `signature` matches the transaction fields and comes from `sender`.
2.  **Verify** the `sender` has sufficient balance.
3.  **Verify** `value` matches the requirement amount.
4.  **Verify** `receiver` matches the requirement `payTo`.
5.  **Verify** `data` field. If expecting a Smart Contract call (e.g. `pay` or `buy`), ensure it contains the correct `resourceId` or item identifier.
6.  **Simulation**: Use `POST /transaction/simulate` to ensure the transaction would succeed.

---

## 2. AssetTransferMethod: `ESDT`

Used for any token on MultiversX.

### Phase 1: `PAYMENT-SIGNATURE` Header Payload

The payload is similar, but the `data` field differs significantly to accommodate the `MultiESDTNFTTransfer` call.

**Structure:**
`MultiESDTNFTTransfer @ <Receiver_Hex> @ <Num_Tokens_Hex> @ <Token_ID_Hex> @ <Nonce_Hex> @ <Amount_Hex> @ <Function_Hex> @ <Args_Hex>...`

**Example PaymentPayload:**

```json
{
  // ... header info ...
  "accepted": {
    "amount": "1000000",
    "asset": "USDC-c70f1a",
    // ...
  },
  "payload": {
    "nonce": 16,
    "value": "0", // EGLD value is 0 for ESDT transfer
    "receiver": "erd1client...", // For MultiESDTNFTTransfer, receiver is Self (Sender)
    "sender": "erd1client...",
    "gasPrice": 1000000000,
    "gasLimit": 60000000, // Higher gas limit
    "data": "MultiESDTNFTTransfer@<dest_hex>@01@<token_hex>@00@<amount_hex>@pay@<resource_id_hex>",
    "chainID": "D",
    "version": 1,
    "validAfter": 1706200000,
    "validBefore": 1706200600
  }
}
```

### Phase 2: Verification Logic

1.  **Verify Signature**.
2.  **Parse Data Field**:
    - Ensure it starts with `MultiESDTNFTTransfer`.
    - Extract Destination, Token Identifier, Amount.
3.  **Verify Extracted Data**:
    - Destination must match `payTo`.
    - Token Identifier must match `asset`.
    - Amount must match required `amount`.
4.  **Simulation**: Essential to catch storage/balance issues.

---

## 3. Relayed Execution

1.  The Facilitator receives the signed payload (Client's Tx).
2.  The Facilitator wraps this in a **Relayed Transaction V3** (Protocol Native).
    - Facilitator submits the tx directly to the network's relayed endpoint.
    - MultiversX natively supports Relayed V3 where the inner transaction is executed and gas is paid by the relayer/facilitator.
    
    *Implementation Note:* The Facilitator constructs the Relayed V3 transaction:
    - Inner Transaction: The signed payload from the client.
    - Relayer: The Facilitator's address (payer of gas).

### Security Note
The Facilitator MUST simulate the transaction before broadcasting to ensure:
1.  It pays the required amount.
2.  It doesn't revert.
3.  It doesn't malicious exploit the relayer (though protocol protections exist).
