import { describe, it, expect, vi } from "vitest";
import { ExactMultiversXScheme } from "../src/exact/client/scheme";
import { MultiversXSigner } from "../src/signer";
import { PaymentRequirements } from "@x402/core/types";

// Mock Signer
const mockSigner = {
    address: "erd1sender",
    signTransaction: vi.fn(async () => "txHash123"),
} as unknown as MultiversXSigner;

describe("ExactMultiversXScheme", () => {
    const scheme = new ExactMultiversXScheme(mockSigner);

    it("should create a valid payment payload", async () => {
        const req: PaymentRequirements = {
            payTo: "erd1sc",
            amount: "1000",
            asset: "EGLD",
            network: "multiversx:1",
            maxTimeoutSeconds: 300,
            extra: {
                resourceId: "res-1"
            }
        };

        const result = await scheme.createPaymentPayload(1, req);

        expect(result.x402Version).toBe(1);
        expect(result.payload.signature).toBe("txHash123");
        expect(result.payload.authorization.resourceId).toBe("res-1");
        expect(result.payload.authorization.to).toBe("erd1sc");

        // Check auto-calculated fields
        const now = Math.floor(Date.now() / 1000);
        expect(Number(result.payload.authorization.validBefore)).toBeGreaterThan(now);
    });

    it("should throw if resourceId is missing", async () => {
        const req: PaymentRequirements = {
            payTo: "erd1sc",
            amount: "1000",
            asset: "EGLD",
            network: "multiversx:1",
            maxTimeoutSeconds: 300
            // missing extra
        };
        await expect(scheme.createPaymentPayload(1, req)).rejects.toThrow("resourceId is required");
    });
});
