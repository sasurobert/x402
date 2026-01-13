import { z } from "zod";

/**
 * Zod schema for the Exact MultiversX Payment details (Authorization).
 * This structure mirrors the fields used in the signature generation.
 */
export const ExactMultiversXAuthorizationSchema = z.object({
    from: z.string(),
    to: z.string(), // Payment SC
    value: z.string(), // Amount in atomic units
    tokenIdentifier: z.string(), // EGLD or TokenID
    resourceId: z.string(), // The Invoice ID (nonce equivalent for protection)
    validAfter: z.string(),
    validBefore: z.string(),
    nonce: z.number().optional(), // Protocol/Account nonce
});

export type ExactMultiversXAuthorization = z.infer<typeof ExactMultiversXAuthorizationSchema>;

export const ExactMultiversXPayloadSchema = z.object({
    authorization: ExactMultiversXAuthorizationSchema,
    signature: z.string(), // The Transaction Hash provided by the signer
});

export type ExactMultiversXPayload = z.infer<typeof ExactMultiversXPayloadSchema>;
