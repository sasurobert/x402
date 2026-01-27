import { x402ResourceServer } from '@x402/core/server'
import { Network } from '@x402/core/types'
import { ExactMultiversXServer } from './scheme'

/**
 * Configuration options for registering MultiversX schemes to an x402ResourceServer
 */
export interface MultiversXResourceServerConfig {
  /**
   * Optional specific networks to register
   * If not provided, registers wildcard support (multiversx:*)
   */
  networks?: Network[]
}

/**
 * Registers MultiversX exact payment schemes to an x402ResourceServer instance.
 *
 * @param server - The x402ResourceServer instance to register schemes to
 * @param config - Configuration for MultiversX resource server registration
 * @returns The server instance for chaining
 */
export function registerExactMultiversXServerScheme(
  server: x402ResourceServer,
  config: MultiversXResourceServerConfig = {},
): x402ResourceServer {
  // Register scheme
  if (config.networks && config.networks.length > 0) {
    // Register specific networks
    config.networks.forEach((network) => {
      server.register(network, new ExactMultiversXServer())
    })
  } else {
    // Register wildcard for all MultiversX chains
    server.register('multiversx:*', new ExactMultiversXServer())
  }

  return server
}
