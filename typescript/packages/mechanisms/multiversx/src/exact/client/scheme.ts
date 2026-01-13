import { PaymentPayload, PaymentRequirements, SchemeNetworkClient } from "@x402/core/types";
import { MultiversXSigner } from "../../signer";
import { ExactMultiversXPayload, ExactMultiversXAuthorization } from "../../types";

/**
 * MultiversX client implementation for the Exact payment scheme.
 */
export class ExactMultiversXScheme implements SchemeNetworkClient {
    readonly scheme = "exact";

    constructor(private readonly signer: MultiversXSigner) { }

    async createPaymentPayload(
        x402Version: number,
        paymentRequirements: PaymentRequirements,
    ): Promise<Pick<PaymentPayload, "x402Version" | "payload">> {
        const now = Math.floor(Date.now() / 1000);

        // We assume 'paymentRequirements.asset' holds the Token Identifier (EGLD or TokenID)
        // The 'payTo' is the SC Address.
        // The 'extra' field contains resourceId.

        if (!paymentRequirements.extra?.resourceId) {
            throw new Error("resourceId is required in payment requirements extra field");
        }

        const authorization: ExactMultiversXAuthorization = {
            from: this.signer.address,
            to: paymentRequirements.payTo,
            value: paymentRequirements.amount,
            tokenIdentifier: paymentRequirements.asset, // asset field used as TokenID
            resourceId: paymentRequirements.extra.resourceId,
            validAfter: (now - 600).toString(),
            validBefore: (now + paymentRequirements.maxTimeoutSeconds).toString(),
            // Nonce is typically fetched by the signer or passed in. 
            // If we need to set it here, we'd need a provider call. 
            // For now, let's leave it undefined and let the Signer fetch it if strictly needed, 
            // or assume the wallet handles nonce management if we pass undefined.
        };

        const chainId = paymentRequirements.network.split(":")[1] || "1";

        // Sign the transaction -> Returns TxHash
        const signature = await this.signer.signTransaction(authorization, chainId);

        const payload: ExactMultiversXPayload = {
            authorization,
            signature,
        };

        return {
            x402Version,
            payload,
        };
    }
}
