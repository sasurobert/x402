import { x402Facilitator } from '@x402/core/facilitator'
import { Network } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXFacilitator } from './scheme'

/**
 * Configuration for the MultiversX facilitator scheme.
 */
export interface MultiversXFacilitatorConfig {
  /** Optional API URL for the MultiversX network */
  apiUrl?: string
  /** Optional signer instance for relativistic transaction relaying */
  signer?: MultiversXSigner
  /** Optional address of the signer */
  signerAddress?: string
  /** Optional list of networks to register against. Defaults to 'multiversx:*' */
  networks?: Network | Network[]
}

/**
 * Registers the Exact MultiversX facilitator scheme with the x402 facilitator.
 *
 * @param facilitator - The x402 facilitator instance
 * @param config - The configuration for the MultiversX scheme
 * @returns The modified facilitator instance
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
