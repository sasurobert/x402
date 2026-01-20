import {
  ValidationResponse,
  PaymentRequirements,
  PaymentPayload,
  SettleResponse,
  SettleResponse,
  ISchemeNetworkFacilitator,
} from '@x402/core'
import { ExactMultiversXPayload } from '../../types'

/**
 * MultiversX Facilitator for Exact scheme.
 */
export class ExactMultiversXFacilitator implements ISchemeNetworkFacilitator {
  /**
   * Creates a new facilitator with API URL.
   *
   * @param apiUrl - The MultiversX API URL (default: devnet)
   */
  constructor(private apiUrl: string = 'https://devnet-api.multiversx.com') {}

  /**
   * Gets the mechanism code.
   *
   * @returns The scheme identifier
   */
  get scheme(): string {
    return 'multiversx-exact-v1'
  }

  /**
   * Gets the CAIP family.
   *
   * @returns The CAIP family string
   */
  get caipFamily(): string {
    return 'multiversx:*'
  }

  /**
   * Gets extra configuration for the network.
   *
   * @param _network - The network identifier
   * @returns Extra config object
   */
  getExtra(_network: string): Record<string, unknown> {
    return {}
  }

  /**
   * Gets list of signers for the network.
   *
   * @param _network - The network identifier
   * @returns Array of signer addresses
   */
  getSigners(_network: string): string[] {
    // Return facilitator wallet addresses if known
    return []
  }

  /**
   * Verifies the payment payload.
   *
   * @param payload - The payment payload
   * @param requirements - The payment requirements
   * @returns Validation response
   */
  async verify(
    payload: PaymentPayload,
    requirements: PaymentRequirements,
  ): Promise<ValidationResponse> {
    // Implement Verification Logic matching Go
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload
    if (!relayedPayload || !relayedPayload.authorization) {
      // If implicit data structure, we might need to parse `data` string?
      // But for now let's assume standard payload structure.
    }

    const data = relayedPayload?.data || relayedPayload
    if (!data) return { isValid: false, reason: 'Missing payload data' }

    const expectedReceiver = requirements.payTo
    const expectedAmount = requirements.amount
    const asset = requirements.asset || 'EGLD'

    // 1. Structural Validation
    if (asset === 'EGLD') {
      if (data.receiver !== expectedReceiver) return { isValid: false, reason: 'Receiver mismatch' }
      // Simple string compare for amount, big int would be better
      if (data.value !== expectedAmount) return { isValid: false, reason: 'Amount mismatch' }
    } else {
      // ESDT check
      if (typeof data.data === 'string' && !data.data.startsWith('MultiESDTNFTTransfer')) {
        return { isValid: false, reason: 'Invalid ESDT data' }
      }
    }

    // 2. Simulation (Simulate via Proxy)
    try {
      const simulationBody = {
        nonce: data.nonce,
        value: data.value,
        receiver: data.receiver,
        sender: data.sender,
        gasPrice: data.gasPrice,
        gasLimit: data.gasLimit,
        data: Buffer.from(data.data || '').toString('base64'),
        chainID: data.chainID,
        version: data.version,
        signature: data.signature,
      }

      const response = await fetch(`${this.apiUrl}/transaction/simulate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(simulationBody),
      })

      if (!response.ok) {
        return { isValid: false, reason: `Simulation API error: ${response.status}` }
      }

      const contentType = response.headers.get('content-type')
      if (!contentType || !contentType.includes('application/json')) {
        // API might return non-JSON if proxy error
        return { isValid: false, reason: 'Invalid simulation response content-type' }
      }

      const simResult = await response.json()
      if (simResult.error) {
        return { isValid: false, reason: `Simulation error: ${simResult.error}` }
      }
      if (simResult.data?.result?.status !== 'success') {
        return {
          isValid: false,
          reason: `Simulation failed:status=${simResult.data?.result?.status}`,
        }
      }

      // Success
      return { isValid: true }
    } catch (e: unknown) {
      const err = e as Error
      return { isValid: false, reason: `Simulation exception: ${err.message}` }
    }
  }

  /**
   * Settles the payment by broadcasting the transaction.
   *
   * @param payload - The payment payload
   * @param _requirements - The payment requirements
   * @returns Settle response
   */
  async settle(
    payload: PaymentPayload,
    _requirements: PaymentRequirements,
  ): Promise<SettleResponse> {
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload
    const data = relayedPayload?.data ? relayedPayload : relayedPayload // Handle structural variations if any

    try {
      // Broadcast
      const txSendBody = {
        nonce: data.nonce,
        value: data.value,
        receiver: data.receiver,
        sender: data.sender,
        gasPrice: data.gasPrice,
        gasLimit: data.gasLimit,
        data: Buffer.from(data.data || '').toString('base64'), // API usually expects base64 for data field if strictly typed or just string?
        // Go sdk uses base64 for simulation. Let's assume same for send.
        signature: data.signature,
        chainID: data.chainID,
        version: data.version,
      }

      const response = await fetch(`${this.apiUrl}/transaction/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(txSendBody),
      })

      if (!response.ok) {
        return { success: false, error: `Broadcast failed: ${response.statusText}` }
      }

      const txResult = await response.json()
      if (txResult.error) {
        return { success: false, error: txResult.error }
      }

      return {
        success: true,
        transaction: txResult.data?.txHash,
      }
    } catch (e: unknown) {
      const err = e as Error
      return { success: false, error: err.message }
    }
  }
}
