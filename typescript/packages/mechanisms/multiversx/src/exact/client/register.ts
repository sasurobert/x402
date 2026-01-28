import { x402Client } from '@x402/core/client'
import { Network } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXScheme } from './scheme'

/**
 * Configuration for the MultiversX client scheme.
 */
export interface MultiversXClientConfig {
  /** The signer instance to use for creating payloads */
  signer: MultiversXSigner
  /** Optional list of networks to register against. Defaults to 'multiversx:*' */
  networks?: Network[]
}

/**
 * Registers the Exact MultiversX client scheme with the x402 client.
 *
 * @param client - The x402 client instance
 * @param config - The configuration for the MultiversX scheme
 * @returns The modified client instance
 */
export function registerExactMultiversXClientScheme(
  client: x402Client,
  config: MultiversXClientConfig,
): x402Client {
  const scheme = new ExactMultiversXScheme(config.signer)

  if (config.networks && config.networks.length > 0) {
    config.networks.forEach((network) => {
      client.register(network, scheme)
    })
  } else {
    client.register('multiversx:*', scheme)
  }

  return client
}
