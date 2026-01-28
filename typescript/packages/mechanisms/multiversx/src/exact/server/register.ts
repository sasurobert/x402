import { x402ResourceServer } from '@x402/core/server'
import { Network } from '@x402/core/types'
import { ExactMultiversXServer } from './scheme'

/**
 * Configuration for the MultiversX resource server scheme.
 */
export interface MultiversXResourceServerConfig {
  /** Optional list of networks to register against. Defaults to 'multiversx:*' */
  networks?: Network[]
}

/**
 * Registers the Exact MultiversX server scheme with the x402 resource server.
 *
 * @param server - The x402 resource server instance
 * @param config - The configuration for the MultiversX scheme
 * @returns The modified server instance
 */
export function registerExactMultiversXServerScheme(
  server: x402ResourceServer,
  config: MultiversXResourceServerConfig = {},
): x402ResourceServer {
  if (config.networks && config.networks.length > 0) {
    config.networks.forEach((network) => {
      server.register(network, new ExactMultiversXServer())
    })
  } else {
    server.register('multiversx:*', new ExactMultiversXServer())
  }

  return server
}
