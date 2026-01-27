import { x402Client } from '@x402/core/client'
import { Network } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXScheme } from './scheme'

/**
 * Configuration options for registering MultiversX schemes to an x402Client
 */
export interface MultiversXClientConfig {
  /**
   * The MultiversX signer instance
   */
  signer: MultiversXSigner
  /**
   * Optional specific networks to register
   * If not provided, registers wildcard support (multiversx:*)
   */
  networks?: Network[]
}

/**
 * Registers MultiversX exact payment schemes to an x402Client instance.
 *
 * @param client - The x402Client instance to register schemes to
 * @param config - Configuration for MultiversX client registration
 * @returns The client instance for chaining
 */
export function registerExactMultiversXClientScheme(
  client: x402Client,
  config: MultiversXClientConfig,
): x402Client {
  const scheme = new ExactMultiversXScheme(config.signer)

  // Register scheme
  if (config.networks && config.networks.length > 0) {
    // Register specific networks
    config.networks.forEach((network) => {
      client.register(network, scheme)
    })
  } else {
    // Register wildcard for all MultiversX chains
    client.register('multiversx:*', scheme)
  }

  return client
}
