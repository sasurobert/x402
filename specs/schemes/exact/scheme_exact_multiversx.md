# Scheme: `exact` on `MultiversX`

## Summary

The `exact` scheme on MultiversX executes a transfer where the Facilitator (server/relayer) pays the gas, while the Client (user) signs the transaction payload. This enables a gasless experience for the user.

This is implemented via two methods depending on the token type:

| AssetTransferMethod | Use Case                                      | Details                                                                 |
| :------------------ | :-------------------------------------------- | :---------------------------------------------------------------------- |
| **1. Direct**       | Native EGLD transfers.                        | Uses a basic transaction where data field calls `pay`.                  |
| **2. ESDT**         | Token transfers (Fungible/SFT/Meta).          | Uses `MultiESDTNFTTransfer` built-in function to send tokens + trigger. |

MultiversX transactions natively support a specific structure (Sender, Receiver, Value, Data, etc.). The "Relayed" v3 architecture allows the Client to sign this structure, and the Facilitator to wrap it in a transaction that pays the gas (via Protocol Relayed Transactions or similar future native support, or simply by the server submitting the signed tx if the user pays gas - though `exact` implies facilitator pays).

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
    // ...
  },
  "accepted": {
    "scheme": "exact",
    "network": "multiversx:D", // "D" for Devnet, "1" for Mainnet, "T" for Testnet
    "amount": "1000000000000000000", // 1 EGLD
    "asset": "EGLD",
    "payTo": "erd1qqqqqqqq...", // Smart Contract or Merchant Address
    "extra": {
      "assetTransferMethod": "direct"
    }
  },
  "payload": {
    "scheme": "multiversx-exact-v1",
    "data": {
      "nonce": 15,
      "value": "1000000000000000000",
      "receiver": "erd1qqqqqqqq...",
      "sender": "erd1client...",
      "gasPrice": 1000000000,
      "gasLimit": 50000,
      "data": "pay@<resource_id_hex>",
      "chainID": "D",
      "version": 1,
      "signature": "a6f... (hex encoded signature)"
    }
  }
}
```

### Phase 2: Verification Logic

1.  **Verify** the `signature` matches the transaction fields and comes from `sender`.
2.  **Verify** the `sender` has sufficient balance.
3.  **Verify** `value` matches the requirement amount.
4.  **Verify** `receiver` matches the requirement `payTo`.
5.  **Verify** `data` field contains the correct `resourceId`.
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
    "scheme": "multiversx-exact-v1",
    "data": {
      // ...
      "value": "0", // EGLD value is 0 for ESDT transfer
      "receiver": "erd1client...", // For MultiESDTNFTTransfer, receiver is Self (Sender)
      "data": "MultiESDTNFTTransfer@<dest_hex>@01@<token_hex>@00@<amount_hex>@pay@<resource_id_hex>",
      "gasLimit": 60000000, // Higher gas limit
      "signature": "..."
    }
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

The Facilitator acts as the Relayer.

1.  The Facilitator receives the signed payload (Client's Tx).
2.  The Facilitator wraps this in a **Relayed Transaction**.
    - **V1 (Smart Contract Relayer)**: Facilitator calls a Relayer SC.
    - **V3 (Protocol Native)**: Facilitator submits the tx directly to the network's relayed endpoint (if available) or wraps it in a protocol-level relayed 
envelope.
    
    *Current Implementation:* We assume the Facilitator submits the transaction to the network. If the transaction is a "Relayed Transaction V1", the Facilitator constructs the wrapper transaction:
    - Sender: Facilitator
    - Receiver: Client (for Relayed V1) or Relayer Hub
    - Data: `relayedTx@<client_nonce>@<client_gas_limit>@<client_gas_price>@<client_receiver>@<client_value>@<client_data>@<client_signature>`
    
    *Recommendation*: Use **Protocol Relayed V1** where the inner transaction is executed and gas is paid by the relayer.

### Security Note
The Facilitator MUST simulate the transaction before broadcasting to ensure:
1.  It pays the required amount.
2.  It doesn't revert.
3.  It doesn't malicious exploit the relayer (though protocol protections exist).
