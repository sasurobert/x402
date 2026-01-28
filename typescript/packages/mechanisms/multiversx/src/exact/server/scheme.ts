import { PaymentRequirements, Price, Network, AssetAmount, MoneyParser } from '@x402/core/types'
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
   * Internal list of custom money parsers.
   */
  private moneyParsers: MoneyParser[] = []

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
   * Registers a custom money parser for specialized price formats.
   *
   * @param parser - The money parser function
   * @returns The server instance for chaining
   */
  registerMoneyParser(parser: MoneyParser): this {
    this.moneyParsers.push(parser)
    return this
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
   * @param price - The price to parse (string, number, or object)
   * @param network - The network context
   * @returns The parsed asset and amount in atomic units
   */
  async parsePrice(price: Price, network: Network): Promise<AssetAmount> {
    if (typeof price === 'object' && price !== null) {
      if ('asset' in price && 'amount' in price) {
        const aa = price as AssetAmount
        if (!aa.asset) {
          throw new Error('asset is required')
        }
        return { asset: aa.asset, amount: aa.amount }
      }

      const p = price as Record<string, unknown>
      const amount = typeof p.amount === 'string' ? p.amount : '0'
      const asset = typeof p.asset === 'string' ? p.asset : ''

      if (!asset) {
        throw new Error('asset is required in price map')
      }

      return { asset, amount }
    }

    const decimalAmount = this.parseMoneyToDecimal(price)

    for (const parser of this.moneyParsers) {
      try {
        const result = await parser(decimalAmount, network)
        if (result) {
          return result
        }
      } catch {
        continue
      }
    }

    return this.defaultMoneyConversion(decimalAmount)
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

  /**
   * Internal helper to parse strings and numbers into a decimal float.
   *
   * @param price - The input price
   * @returns The parsed numeric value
   * @throws Error if the price format is unsupported
   */
  private parseMoneyToDecimal(price: Price): number {
    if (typeof price === 'number') {
      return price
    }

    if (typeof price === 'string') {
      let cleanPrice = price.trim()
      cleanPrice = cleanPrice.replace(/^\$/, '')
      cleanPrice = cleanPrice.replace(/ USD$/, '')
      cleanPrice = cleanPrice.replace(/ USDC$/, '')
      cleanPrice = cleanPrice.trim()

      const amount = parseFloat(cleanPrice)
      if (isNaN(amount)) {
        throw new Error(`failed to parse price string '${price}'`)
      }
      return amount
    }

    throw new Error(`unsupported price type: ${typeof price}`)
  }

  /**
   * Internal helper to convert a decimal amount to atomic EGLD units.
   *
   * @param amount - The decimal amount (e.g., 1.5)
   * @returns AssetAmount with EGLD and 18-decimal padded string
   */
  private defaultMoneyConversion(amount: number): AssetAmount {
    const decimals = 18
    const cleanAmount = amount.toString()

    let intPart: string
    let decPart: string

    if (cleanAmount.includes('.')) {
      const parts = cleanAmount.split('.')
      intPart = parts[0]
      decPart = parts[1].padEnd(decimals, '0').slice(0, decimals)
    } else {
      intPart = cleanAmount
      decPart = '0'.repeat(decimals)
    }

    let finalAmount = (intPart + decPart).replace(/^0+/, '')
    if (finalAmount === '') finalAmount = '0'

    return {
      asset: 'EGLD',
      amount: finalAmount,
    }
  }
}
