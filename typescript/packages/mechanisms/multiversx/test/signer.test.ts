import { describe, it, expect, vi } from "vitest";
import { MultiversXSigner, ISignerProvider } from "../src/signer";
import { Transaction, Address } from "@multiversx/sdk-core";

// Mock Provider
const mockProvider = {
        signTransaction: vi.fn(async (tx: Transaction) => {
                // Return the tx as is, getHash works on unsigned too
                return tx;
        }),
        getAddress: vi.fn(async () => "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th"),
} as unknown as ISignerProvider;

describe("MultiversXSigner", () => {
        // Use known valid addresses generated from zero/one buffers
        const alice = new Address(Buffer.alloc(32, 1)).bech32();
        const bob = new Address(Buffer.alloc(32, 2)).bech32();

        const signer = new MultiversXSigner(mockProvider, alice);

        it("should construct a correct EGLD transaction", async () => {
                const request = {
                        to: bob,
                        amount: "1000000000000000000", // 1 EGLD
                        tokenIdentifier: "EGLD",
                        resourceId: "invoice-123",
                        chainId: "1",
                        nonce: 5
                };

                await signer.sign(request);

                expect(mockProvider.signTransaction).toHaveBeenCalled();
                // Get the transaction object passed to the mock
                const tx = (mockProvider.signTransaction as any).mock.calls[0][0] as Transaction;

                expect(tx.getReceiver().bech32()).toBe(bob);
                expect(tx.getValue().toString()).toBe("1000000000000000000");
                // Expect exact resourceId in data (no pay@)
                expect(tx.getData().toString()).toBe("invoice-123");
                expect(tx.getGasLimit().valueOf()).toBe(50_000);
        });

        it("should construct a correct ESDT transaction", async () => {
                (mockProvider.signTransaction as any).mockClear();

                const request = {
                        to: bob,
                        amount: "500", // 500 atomic units
                        tokenIdentifier: "TOKEN-123456",
                        resourceId: "invoice-456",
                        chainId: "D"
                };

                await signer.sign(request);

                const tx = (mockProvider.signTransaction as any).mock.calls[0][0] as Transaction;

                expect(tx.getReceiver().bech32()).toBe(alice);
                expect(tx.getValue().toString()).toBe("0");

                const data = tx.getData().toString();
                expect(data).toContain("MultiESDTNFTTransfer");
                expect(data).toContain(new Address(bob).hex());
                expect(data).toContain(Buffer.from("TOKEN-123456").toString('hex'));
                // Expect resourceId hex at the end (as function name)
                expect(data).toContain(Buffer.from("invoice-456").toString('hex'));
        });

        it("should construct a MultiESDT transaction for EGLD-000000", async () => {
                (mockProvider.signTransaction as any).mockClear();

                const request = {
                        to: bob,
                        amount: "1000",
                        tokenIdentifier: "EGLD-000000", // Should trigger ESDT path
                        resourceId: "item-789",
                        chainId: "D"
                };

                await signer.sign(request);

                const tx = (mockProvider.signTransaction as any).mock.calls[0][0] as Transaction;

                // ESDT Path: Receiver = Sender
                expect(tx.getReceiver().bech32()).toBe(alice);
                expect(tx.getValue().toString()).toBe("0");

                const data = tx.getData().toString();
                expect(data).toContain("MultiESDTNFTTransfer");
                // Token Identifier "EGLD-000000" should be hex encoded in data
                expect(data).toContain(Buffer.from("EGLD-000000").toString('hex'));
                expect(data).toContain(Buffer.from("item-789").toString('hex'));
        });
});
