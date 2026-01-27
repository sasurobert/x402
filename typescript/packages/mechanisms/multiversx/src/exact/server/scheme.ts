import { PaymentRequirements, Price, Network, AssetAmount } from '@x402/core/types'
import { SchemeNetworkServer } from '@x402/core/types/mechanisms'

/**
 * MultiversX Server implementation.
 */
export class ExactMultiversXServer implements SchemeNetworkServer {
  /**
   * Gets the scheme identifier.
   *
   * @returns The scheme string
   */
  get scheme(): string {
    return 'exact'
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
  async parsePrice(price: Price, _network: Network): Promise<AssetAmount> {
    // Handle Price parsing similar to Go "ParsePrice"
    // Expect object with amount/asset or string
    let amount = '0'
    let asset = 'EGLD'

    if (typeof price === 'object' && price !== null) {
      const p = price as Record<string, unknown>
      amount = typeof p.amount === 'string' ? p.amount : '0'
      asset = typeof p.asset === 'string' ? p.asset : 'EGLD'
    } else if (typeof price === 'string') {
      // Handle dollar format like "$0.001"
      const cleanPrice = price.replace(/^\$/, '').trim()
      const numericValue = parseFloat(cleanPrice)

      if (!isNaN(numericValue)) {
        // Convert USD to EGLD base units (18 decimals)
        // For simplicity in E2E testing, treat the numeric value as EGLD amount
        // In production, you would use an oracle to get USD/EGLD rate
        const egldDecimals = 18
        const [intPart, decPart = ''] = cleanPrice.split('.')
        const paddedDec = decPart.padEnd(egldDecimals, '0').slice(0, egldDecimals)
        amount = intPart + paddedDec
        // Remove leading zeros but keep at least one digit
        amount = amount.replace(/^0+/, '') || '0'
      } else {
        amount = cleanPrice
      }
    }

    return { asset, amount }
  }

  /**
   * Enhances requirements with defaults.
   *
   * @param requirements - Input requirements
   * @param _supportedKind - The supported kind configuration (unused)
   * @param _supportedKind.x402Version - The x402 version (unused)
   * @param _supportedKind.scheme - The scheme identifier (unused)
   * @param _supportedKind.network - The network identifier (unused)
   * @param _supportedKind.extra - Extra configuration (unused)
   * @param _facilitatorExtensions - List of facilitator extensions (unused)
   * @returns Enhanced requirements
   */
  async enhancePaymentRequirements(
    requirements: PaymentRequirements,
    _supportedKind: {
      x402Version: number
      scheme: string
      network: Network
      extra?: Record<string, unknown>
    },
    _facilitatorExtensions: string[],
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
}
