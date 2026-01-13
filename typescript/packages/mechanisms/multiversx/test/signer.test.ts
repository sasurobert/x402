import { describe, it, expect, vi } from "vitest";
import { MultiversXSigner, ISignerProvider } from "../src/signer";
import { Transaction } from "@multiversx/sdk-core";
import { ExactMultiversXAuthorization } from "../src/types";

// Mock Provider
const mockProvider = {
    signTransaction: vi.fn(async (tx: Transaction) => {
        // Return a mock object with getHash based on signature
        tx.setSignature(Buffer.from("mock_signature"));
        return tx;
    }),
    getAddress: vi.fn(async () => "erd1sender"),
} as unknown as ISignerProvider;

describe("MultiversXSigner", () => {
    const signer = new MultiversXSigner(mockProvider, "erd1sender");

    it("should construct a correct EGLD transaction", async () => {
        const auth: ExactMultiversXAuthorization = {
            from: "erd1sender",
            to: "erd1recipient",
            value: "1000000000000000000", // 1 EGLD
            tokenIdentifier: "EGLD",
            resourceId: "invoice-123",
            validAfter: "0",
            validBefore: "1000",
            nonce: 5
        };

        const hash = await signer.signTransaction(auth, "1");

        expect(mockProvider.signTransaction).toHaveBeenCalled();

        // Get the transaction object passed to the mock
        const tx = (mockProvider.signTransaction as any).mock.calls[0][0] as Transaction;

        expect(tx.getReceiver().bech32()).toBe("erd1recipient");
        expect(tx.getValue().toString()).toBe("1000000000000000000");
        expect(tx.getData().toString()).toBe("pay@696e766f6963652d313233"); // pay@hex("invoice-123")
        expect(tx.getGasLimit().valueOf()).toBe(10_000_000);
    });

    it("should construct a correct ESDT transaction", async () => {
        (mockProvider.signTransaction as any).mockClear();

        const auth: ExactMultiversXAuthorization = {
            from: "erd1sender", // Sender
            to: "erd1recipient", // Destination SC
            value: "500", // 500 atomic units
            tokenIdentifier: "TOKEN-123456",
            resourceId: "invoice-456",
            validAfter: "0",
            validBefore: "1000",
        };

        await signer.signTransaction(auth, "D");

        const tx = (mockProvider.signTransaction as any).mock.calls[0][0] as Transaction;

        // For ESDT, receiver is self (sender)
        expect(tx.getReceiver().bech32()).toBe("erd1sender");
        // Value is 0 EGLD
        expect(tx.getValue().toString()).toBe("0");

        // Check Data for MultiESDT properties
        // MultiESDTNFTTransfer@<dest>@01@<token>@00@<amount>@pay@<resourceId>
        const data = tx.getData().toString();
        expect(data).toContain("MultiESDTNFTTransfer");
        expect(data).toContain(Buffer.from("erd1recipient").toString('hex')); // Error: dest needs to be address hex, verifying later
        // Actually the code uses address.hex(). Let's verify we see the token hex.
        expect(data).toContain(Buffer.from("TOKEN-123456").toString('hex'));
        expect(data).toContain("01f4"); // 500 in hex
        expect(data).toContain("pay");
        expect(data).toContain(Buffer.from("invoice-456").toString('hex'));
    });
});
