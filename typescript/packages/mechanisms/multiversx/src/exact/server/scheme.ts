import {
    PaymentPayload,
    PaymentRequirements,
    ISchemeNetworkServer
} from "@x402/core";

export class ExactMultiversXServer implements ISchemeNetworkServer {

    get scheme(): string {
        return "multiversx-exact-v1";
    }

    get caipFamily(): string {
        return "multiversx:*";
    }

    getExtra(network: string): Record<string, any> {
        return {};
    }

    async createPaymentPayload(requirements: PaymentRequirements): Promise<PaymentPayload> {
        // Server usually constructs the payload for the client to sign?
        // In "Exact" scheme, Client constructs it.
        // But if Server acts as Coordinator, it might prepare args.
        // Return empty/mock for now as logic is primarily Client-Signer.
        return {
            x402Version: 2,
            payload: {}
        };
    }
}
