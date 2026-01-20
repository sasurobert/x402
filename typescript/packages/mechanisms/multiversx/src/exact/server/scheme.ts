import { PaymentPayload, PaymentRequirements, ISchemeNetworkServer } from '@x402/core'

/**
 * MultiversX Server implementation.
 */
export class ExactMultiversXServer implements ISchemeNetworkServer {
  /**
   * Gets the scheme identifier.
   *
   * @returns The scheme string
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
   * Gets extra config.
   *
   * @param _network - The network identifier
   * @returns Extra config object
   */
  getExtra(_network: string): Record<string, unknown> {
    return {}
  }

  /**
   * Parses the price.
   *
   * @param price - The raw price object or string
   * @param _network - The network identifier
   * @returns Parse asset and amount
   */
  async parsePrice(price: unknown, _network: string): Promise<{ asset: string; amount: string }> {
    // Handle Price parsing similar to Go "ParsePrice"
    // Expect object with amount/asset or string
    let amount = '0'
    let asset = 'EGLD'

    if (typeof price === 'object' && price !== null) {
      const p = price as Record<string, unknown>
      amount = typeof p.amount === 'string' ? p.amount : '0'
      asset = typeof p.asset === 'string' ? p.asset : 'EGLD'
    } else if (typeof price === 'string') {
      amount = price // Assume EGLD if just amount string? Or error?
    }

    return { asset, amount }
  }

  /**
   * Enhances requirements with defaults.
   *
   * @param requirements - Input requirements
   * @returns Enhanced requirements   */
  async enhancePaymentRequirements(
    requirements: PaymentRequirements,
  ): Promise<PaymentRequirements> {
    // Add defaults
    const req = { ...requirements }

    if (!req.extra) {
      req.extra = {}
    }

    if (!req.asset) {
      req.asset = 'EGLD'
    }

    if (!req.payTo) {
      throw new Error('PayTo is required for MultiversX payments')
    }

    return req
  }

  /**
   * Creates a payment payload (Server-side).
   *
   * @param _requirements - The payment requirements
   * @returns The payment payload
   */
  async createPaymentPayload(_requirements: PaymentRequirements): Promise<PaymentPayload> {
    return {
      x402Version: 2,
      payload: {},
    }
  }
}
