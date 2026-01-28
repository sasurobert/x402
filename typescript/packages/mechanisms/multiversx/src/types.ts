import { z } from 'zod'

/**
 * Zod schema for the MultiversX authorization object.
 * This represents the fields used for offline signing or verification.
 */
export const ExactMultiversXAuthorizationSchema = z.object({
  /** The sender's bech32 address */
  from: z.string(),
  /** The recipient's bech32 address */
  to: z.string(),
  /** The value to transfer in atomic units */
  value: z.string(),
  /** The token identifier (EGLD or ESDT ID) */
  tokenIdentifier: z.string(),
  /** Unique resource identifier for the payment */
  resourceId: z.string(),
  /** Timestamp after which the authorization is valid */
  validAfter: z.string(),
  /** Timestamp before which the authorization is valid */
  validBefore: z.string(),
  /** Optional transaction nonce */
  nonce: z.number().optional(),
})

/** MultiversX authorization type derived from the Zod schema */
export type ExactMultiversXAuthorization = z.infer<typeof ExactMultiversXAuthorizationSchema>

/**
 * Zod schema for the MultiversX payment payload.
 * This represents the actual transaction data relayed to the network.
 */
export const ExactMultiversXPayloadSchema = z.object({
  /** Transaction nonce */
  nonce: z.number(),
  /** Transaction value in atomic units */
  value: z.string(),
  /** Recipient address (bech32) */
  receiver: z.string(),
  /** Sender address (bech32) */
  sender: z.string(),
  /** Transaction gas price */
  gasPrice: z.number(),
  /** Transaction gas limit */
  gasLimit: z.number(),
  /** Optional transaction data (base64 or string depending on context) */
  data: z.string().optional(),
  /** Network chain ID */
  chainID: z.string(),
  /** Transaction version */
  version: z.number(),
  /** Optional transaction options */
  options: z.number().optional(),
  /** Transaction signature (hex) */
  signature: z.string().optional(),
  /** Optional relayer address (bech32) */
  relayer: z.string().optional(),
  /** Optional relayer signature (hex) */
  relayerSignature: z.string().optional(),
  /** Timestamp after which the payload is valid */
  validAfter: z.number().optional(),
  /** Timestamp before which the payload is valid */
  validBefore: z.number().optional(),
})

/** MultiversX payment payload type derived from the Zod schema */
export type ExactMultiversXPayload = z.infer<typeof ExactMultiversXPayloadSchema>
