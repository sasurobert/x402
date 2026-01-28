import { PaymentRequirements, Price, Network, AssetAmount } from '@x402/core/types'
import { SchemeNetworkServer } from '@x402/core/types/mechanisms'
import {
  MULTIVERSX_GAS_LIMIT_EGLD,
  MULTIVERSX_GAS_LIMIT_ESDT,
  MULTIVERSX_TRANSFER_METHOD_DIRECT,
  MULTIVERSX_TRANSFER_METHOD_ESDT,
} from '../../constants'

/**
 * MultiversX Server implementation for the Exact payment scheme.
 */
export class ExactMultiversXServer implements SchemeNetworkServer {
  /**
   * The scheme identifier for this server.
   *
   * @returns The string 'exact'
   */
  get scheme(): string {
    return 'exact'
  }

  /**
   * The CAIP-compatible family identifier for MultiversX.
   *
   * @returns The wildcard 'multiversx:*'
   */
  get caipFamily(): string {
    return 'multiversx:*'
  }

  /**
   * Gets extra configuration for a specific network.
   *
   * @param _network - The network identifier
   * @returns An empty record as no extra config is needed by default
   */
  getExtra(_network: string): Record<string, unknown> {
    return {}
  }

  /**
   * Parses various price formats into atomic units for MultiversX.
   *
   * @param price - The price to parse (string or object)
   * @param _network - The network context
   * @returns The parsed asset and amount in atomic units
   */
  async parsePrice(price: Price, _network: Network): Promise<AssetAmount> {
    let amount = '0'
    let asset = 'EGLD'

    if (typeof price === 'object' && price !== null) {
      const p = price as Record<string, unknown>
      amount = typeof p.amount === 'string' ? p.amount : '0'
      asset = typeof p.asset === 'string' ? p.asset : 'EGLD'
    } else if (typeof price === 'string') {
      const cleanPrice = price.replace(/^\$/, '').trim()
      const numericValue = parseFloat(cleanPrice)

      if (!isNaN(numericValue)) {
        const egldDecimals = 18
        const [intPart, decPart = ''] = cleanPrice.split('.')
        const paddedDec = decPart.padEnd(egldDecimals, '0').slice(0, egldDecimals)
        amount = intPart + paddedDec
        amount = amount.replace(/^0+/, '') || '0'
      } else {
        amount = cleanPrice
      }
    }

    return { asset, amount }
  }

  /**
   * Enhances payment requirements with default MultiversX-specific fields.
   *
   * @param requirements - The original payment requirements
   * @param _supportedKind - The supported kind metadata
   * @param _supportedKind.x402Version - The version of x402
   * @param _supportedKind.scheme - The payment scheme
   * @param _supportedKind.network - The network
   * @param _supportedKind.extra - Optional extra info
   * @param _facilitatorExtensions - Array of facilitator extensions
   * @returns The enhanced payment requirements
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
    this.validatePaymentRequirements(requirements)

    const req = { ...requirements }

    if (req.extra) {
      req.extra = { ...req.extra }
    } else {
      req.extra = {}
    }

    if (!req.extra.assetTransferMethod) {
      if (req.asset === 'EGLD') {
        req.extra.assetTransferMethod = MULTIVERSX_TRANSFER_METHOD_DIRECT
      } else {
        req.extra.assetTransferMethod = MULTIVERSX_TRANSFER_METHOD_ESDT
      }
    }

    if (!req.extra.gasLimit) {
      if (req.extra.assetTransferMethod === MULTIVERSX_TRANSFER_METHOD_DIRECT) {
        req.extra.gasLimit = MULTIVERSX_GAS_LIMIT_EGLD
      } else {
        req.extra.gasLimit = MULTIVERSX_GAS_LIMIT_ESDT
      }
    }

    return req
  }

  /**
   * Validates that the payment requirements meet MultiversX standards.
   *
   * @param requirements - The requirements to validate
   * @throws Error if any requirement is invalid
   */
  validatePaymentRequirements(requirements: PaymentRequirements): void {
    if (!requirements.payTo) {
      throw new Error('PayTo is required for MultiversX payments')
    }

    if (requirements.payTo.length !== 62 || !requirements.payTo.startsWith('erd1')) {
      throw new Error(`invalid PayTo address: ${requirements.payTo}`)
    }

    if (!requirements.amount) {
      throw new Error('amount is required')
    }

    if (!/^\d+$/.test(requirements.amount)) {
      throw new Error(`invalid amount: ${requirements.amount}`)
    }

    if (!requirements.asset) {
      throw new Error('asset is required')
    }

    if (requirements.asset !== 'EGLD') {
      if (!/^[A-Z0-9]{3,8}-[0-9a-fA-F]{6}$/.test(requirements.asset)) {
        throw new Error(`invalid asset TokenID: ${requirements.asset}`)
      }
    }
  }
}
