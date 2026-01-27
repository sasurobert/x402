import { x402Facilitator } from '@x402/core/facilitator'
import { Network } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXFacilitator } from './scheme'

/**
 * Configuration options for registering MultiversX facilitator schemes
 */
export interface MultiversXFacilitatorConfig {
  /**
   * The MultiversX API URL
   */
  apiUrl?: string
  /**
   * Optional signer for Relayed V3 transactions
   */
  signer?: MultiversXSigner
  /**
   * Optional address of the signer
   */
  signerAddress?: string
  /**
   * Optional specific networks to register
   * If not provided, registers wildcard support (multiversx:*)
   */
  networks?: Network | Network[]
}

/**
 * Registers MultiversX exact payment schemes to an x402Facilitator instance.
 *
 * @param facilitator - The x402Facilitator instance to register schemes to
 * @param config - Configuration for MultiversX facilitator registration
 * @returns The facilitator instance for chaining
 */
export function registerExactMultiversXFacilitatorScheme(
  facilitator: x402Facilitator,
  config: MultiversXFacilitatorConfig = {},
): x402Facilitator {
  const scheme = new ExactMultiversXFacilitator(config.apiUrl, config.signer, config.signerAddress)

  const networks = config.networks
    ? Array.isArray(config.networks)
      ? config.networks
      : [config.networks]
    : ['multiversx:*']

  networks.forEach((network) => {
    facilitator.register(network as Network, scheme)
  })

  return facilitator
}
