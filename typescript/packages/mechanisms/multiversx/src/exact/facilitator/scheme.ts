import {
  VerifyResponse,
  PaymentRequirements,
  PaymentPayload,
  SettleResponse,
  Network,
} from '@x402/core/types'
import { SchemeNetworkFacilitator } from '@x402/core/types/mechanisms'
import { ExactMultiversXPayload } from '../../types'
import { MultiversXSigner } from '../../signer'
import { Transaction, Address, TransactionPayload } from '@multiversx/sdk-core'
import { ApiNetworkProvider } from '@multiversx/sdk-network-providers'
import {
  CHAIN_ID_DEVNET,
  CHAIN_ID_MAINNET,
  CHAIN_ID_TESTNET,
  MULTIVERSX_API_URL_DEVNET,
  MULTIVERSX_API_URL_MAINNET,
  MULTIVERSX_API_URL_TESTNET,
  MULTIVERSX_METHOD_MULTI_TRANSFER,
  MULTIVERSX_NATIVE_TOKEN,
} from '../../constants'

/**
 * MultiversX Facilitator implementation for the Exact payment scheme.
 */
export class ExactMultiversXFacilitator implements SchemeNetworkFacilitator {
  /**
   * Initializes the ExactMultiversXFacilitator.
   *
   * @param apiUrl - The MultiversX API URL to use for simulations and status checks
   * @param signer - Optional MultiversX signer for relaying transactions
   * @param signerAddress - Optional address of the signer
   */
  constructor(
    private apiUrl: string = MULTIVERSX_API_URL_DEVNET,
    private signer?: MultiversXSigner,
    private signerAddress?: string,
  ) { }

  /**
   * The scheme identifier for this facilitator.
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
  getExtra(_network: Network): Record<string, unknown> {
    return {}
  }

  /**
   * Gets the list of addresses authorized to sign or facilitate for a network.
   *
   * @param _network - The network identifier
   * @returns Array containing the signer address if provided
   */
  getSigners(_network: Network | string): string[] {
    if (this.signerAddress) return [this.signerAddress]
    return []
  }

  /**
   * Verifies that a payment payload matches the requirements and is cryptographically valid.
   *
   * @param payload - The payment payload to verify
   * @param requirements - The original payment requirements to match against
   * @returns A response indicating if the payload is valid
   */
  async verify(
    payload: PaymentPayload,
    requirements: PaymentRequirements,
  ): Promise<VerifyResponse> {
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload
    const now = Math.floor(Date.now() / 1000)

    if (
      relayedPayload.validBefore &&
      relayedPayload.validBefore > 0 &&
      now > relayedPayload.validBefore
    ) {
      return {
        isValid: false,
        invalidReason: `Payment expired (validBefore: ${relayedPayload.validBefore}, now: ${now})`,
      }
    }
    if (
      relayedPayload.validAfter &&
      relayedPayload.validAfter > 0 &&
      now < relayedPayload.validAfter
    ) {
      return {
        isValid: false,
        invalidReason: `Payment not yet valid (validAfter: ${relayedPayload.validAfter}, now: ${now})`,
      }
    }

    const expectedReceiver = requirements.payTo
    const expectedAmount = requirements.amount
    const asset = requirements.asset || MULTIVERSX_NATIVE_TOKEN

    if (asset !== MULTIVERSX_NATIVE_TOKEN) {
      if (relayedPayload.receiver !== relayedPayload.sender) {
        return {
          isValid: false,
          invalidReason: `Receiver mismatch for ESDT (expected self-send to ${relayedPayload.sender}, got ${relayedPayload.receiver})`,
        }
      }

      const parts = (relayedPayload.data || '').split('@')
      if (parts.length < 6 || parts[0] !== MULTIVERSX_METHOD_MULTI_TRANSFER) {
        return { isValid: false, invalidReason: 'Invalid ESDT transfer data format' }
      }

      const destHex = parts[1]
      const destAddr = Address.newFromBech32(expectedReceiver)
      if (destHex !== destAddr.hex()) {
        return {
          isValid: false,
          invalidReason: `Receiver mismatch in data (expected ${expectedReceiver}, got hex ${destHex})`,
        }
      }

      const tokenHex = parts[3]
      const tokenStr = Buffer.from(tokenHex, 'hex').toString('utf8')
      if (tokenStr !== asset) {
        return {
          isValid: false,
          invalidReason: `Asset mismatch (expected ${asset}, got ${tokenStr})`,
        }
      }

      const amountHex = parts[5]
      const amountBi = BigInt('0x' + amountHex)
      const expectedBi = BigInt(expectedAmount)
      if (amountBi < expectedBi) {
        return {
          isValid: false,
          invalidReason: `Amount too low (expected ${expectedAmount}, got ${amountBi.toString()})`,
        }
      }
    } else {
      if (relayedPayload.receiver !== expectedReceiver) {
        return {
          isValid: false,
          invalidReason: `Receiver mismatch (expected ${expectedReceiver}, got ${relayedPayload.receiver})`,
        }
      }

      const valBi = BigInt(relayedPayload.value)
      const expBi = BigInt(expectedAmount)
      if (valBi < expBi) {
        return {
          isValid: false,
          invalidReason: `Amount too low (expected ${expectedAmount}, got ${valBi.toString()})`,
        }
      }
    }

    const signatureValid = await this.verifySignature(relayedPayload)
    if (!signatureValid.isValid) {
      return signatureValid
    }

    const apiUrl = this.resolveApiUrl(requirements.network)
    const provider = new ApiNetworkProvider(apiUrl)
    return this.verifyViaSimulation(relayedPayload, provider)
  }

  /**
   * Broadcasts the payment transaction to the MultiversX network.
   *
   * @param payload - The payment payload to settle
   * @param requirements - The requirements for the payment
   * @returns A response indicating if the settlement was successful and the transaction hash
   */
  async settle(
    payload: PaymentPayload,
    requirements: PaymentRequirements,
  ): Promise<SettleResponse> {
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload
    const network = requirements.network as Network
    const payer = relayedPayload.sender
    const apiUrl = this.resolveApiUrl(network)
    const provider = new ApiNetworkProvider(apiUrl)

    try {
      const transaction = new Transaction({
        nonce: BigInt(relayedPayload.nonce),
        value: relayedPayload.value,
        receiver: Address.newFromBech32(relayedPayload.receiver),
        sender: Address.newFromBech32(relayedPayload.sender),
        gasPrice: BigInt(relayedPayload.gasPrice),
        gasLimit: relayedPayload.gasLimit,
        data: new TransactionPayload(relayedPayload.data),
        chainID: relayedPayload.chainID,
        version: relayedPayload.version,
        options: relayedPayload.options,
      })

      transaction.applySignature(Buffer.from(relayedPayload.signature || '', 'hex'))

      if (relayedPayload.relayer) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ; (transaction as any).relayer = Address.newFromBech32(relayedPayload.relayer)
        if (relayedPayload.relayerSignature) {
          transaction.applySignature(Buffer.from(relayedPayload.relayerSignature, 'hex'))
        }
      }

      const txHash = await provider.sendTransaction(transaction)

      if (!txHash) {
        return {
          success: false,
          errorReason: 'Broadcast succeeded but no hash returned',
          transaction: '',
          network,
          payer,
        }
      }

      try {
        await this.waitForTx(txHash, provider)
      } catch (waitError: unknown) {
        const err = waitError as Error
        return {
          success: false,
          errorReason: `Transaction broadcasted but wait failed: ${err.message}`,
          transaction: txHash,
          network,
          payer,
        }
      }

      return {
        success: true,
        transaction: txHash,
        network,
        payer,
      }
    } catch (e: unknown) {
      const err = e as Error
      return { success: false, errorReason: err.message, transaction: '', network, payer }
    }
  }

  /**
   * Performs an on-chain simulation of the transaction to verify its validity.
   *
   * @param payload - The payload to simulate
   * @param provider - The MultiversX API provider to use for simulations
   * @returns A verification response based on simulation results
   */
  private async verifyViaSimulation(
    payload: ExactMultiversXPayload,
    provider: ApiNetworkProvider,
  ): Promise<VerifyResponse> {
    try {
      const transaction = new Transaction({
        nonce: BigInt(payload.nonce),
        value: payload.value,
        receiver: Address.newFromBech32(payload.receiver),
        sender: Address.newFromBech32(payload.sender),
        gasLimit: payload.gasLimit,
        gasPrice: BigInt(payload.gasPrice),
        data: new TransactionPayload(payload.data),
        chainID: payload.chainID,
        version: payload.version,
        options: payload.options,
      })

      transaction.applySignature(Buffer.from(payload.signature || '', 'hex'))

      const relayerAddr = payload.relayer
      let relayerSig = payload.relayerSignature

      if (this.signer && this.signerAddress && relayerAddr === this.signerAddress && !relayerSig) {
        if (relayerAddr) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          ; (transaction as any).relayer = Address.newFromBech32(relayerAddr)
        }
        relayerSig = await this.signer.signTransaction(transaction)
      }

      if (relayerAddr) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ; (transaction as any).relayer = Address.newFromBech32(relayerAddr)
        if (relayerSig) {
          transaction.applySignature(Buffer.from(relayerSig, 'hex'))
        }
      }

      const simResult = await provider.simulateTransaction(transaction)

      if (simResult.error) {
        return { isValid: false, invalidReason: `Simulation error: ${simResult.error}` }
      }

      const status = simResult.status
      if (status !== 'success' && status !== 'successful') {
        return {
          isValid: false,
          invalidReason: `Simulation failed: ${simResult.returnMessage || status}`,
        }
      }

      return { isValid: true }
    } catch (e: unknown) {
      const err = e as Error
      return { isValid: false, invalidReason: `Simulation exception: ${err.message}` }
    }
  }

  /**
   * Verifies the Ed25519 signature of the transaction.
   *
   * @param payload - The payload containing the signature and transaction data
   * @returns A verification response based on the cryptographic check
   */
  private async verifySignature(payload: ExactMultiversXPayload): Promise<VerifyResponse> {
    try {
      if (!payload.signature) {
        return { isValid: false, invalidReason: 'Missing signature' }
      }

      const tx = new Transaction({
        nonce: BigInt(payload.nonce),
        value: payload.value,
        receiver: Address.newFromBech32(payload.receiver),
        sender: Address.newFromBech32(payload.sender),
        gasLimit: payload.gasLimit,
        gasPrice: BigInt(payload.gasPrice),
        data: new TransactionPayload(payload.data || ''),
        chainID: payload.chainID,
        version: payload.version,
        options: payload.options,
      })

      const { TransactionComputer } = await import('@multiversx/sdk-core')
      const txComputer = new TransactionComputer()
      const serializedTx = txComputer.computeBytesForSigning(tx)

      const ed = await import('@noble/ed25519')

      const senderAddress = Address.newFromBech32(payload.sender)
      const publicKeyBytes = senderAddress.getPublicKey()
      const signatureBytes = Buffer.from(payload.signature, 'hex')

      const isValid = await ed.verifyAsync(signatureBytes, serializedTx, publicKeyBytes)

      if (!isValid) {
        return { isValid: false, invalidReason: 'Invalid Ed25519 signature' }
      }

      return { isValid: true, payer: payload.sender }
    } catch (e: unknown) {
      const err = e as Error
      return { isValid: false, invalidReason: `Signature verification failed: ${err.message}` }
    }
  }

  /**
   * Status check helper that waits until a transaction is finalized or fails.
   *
   * @param txHash - The hash of the transaction to wait for
   * @param provider - The MultiversX API provider to use for waiting
   * @throws Error if the transaction fails
   */
  private async waitForTx(txHash: string, provider: ApiNetworkProvider): Promise<void> {
    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await (provider as any).awaitTransactionCompleted(txHash)
    } catch (e: unknown) {
      const err = e as Error
      throw new Error(`Transaction failed or wait timed out: ${err.message}`)
    }
  }

  /**
   * Resolves the MultiversX API URL for a given network identifier.
   *
   * @param network - The network identifier (e.g., 'multiversx:1', 'multiversx:D')
   * @returns The resolved API URL
   */
  private resolveApiUrl(network: string): string {
    const chainId = network.split(':')[1]
    switch (chainId) {
      case CHAIN_ID_MAINNET:
        return MULTIVERSX_API_URL_MAINNET
      case CHAIN_ID_DEVNET:
        return MULTIVERSX_API_URL_DEVNET
      case CHAIN_ID_TESTNET:
        return MULTIVERSX_API_URL_TESTNET
      default:
        return this.apiUrl
    }
  }
}
