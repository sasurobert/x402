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

/**
 * MultiversX Facilitator for Exact scheme.
 */
export class ExactMultiversXFacilitator implements SchemeNetworkFacilitator {
  /**
   * Creates a new facilitator with API URL.
   *
   * @param apiUrl - The MultiversX API URL (default: devnet)
   * @param signer - Optional signer for Relayed V3 transactions
   * @param signerAddress - Optional address of the signer
   */
  constructor(
    private apiUrl: string = 'https://devnet-api.multiversx.com',
    private signer?: MultiversXSigner,
    private signerAddress?: string,
  ) { }

  /**
   * Gets the mechanism code.
   *
   * @returns The scheme identifier
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
   * Gets extra configuration for the network.
   *
   * @param _network - The network identifier
   * @returns Extra config object
   */
  getExtra(_network: Network): Record<string, unknown> {
    return {}
  }

  /**
   * Gets list of signers for the network.
   *
   * @param _network - The network identifier
   * @returns Array of signer addresses
   */
  getSigners(_network: Network | string): string[] {
    if (this.signerAddress) return [this.signerAddress]
    return []
  }

  /**
   * Verifies the payment payload.
   *
   * @param payload - The payment payload
   * @param requirements - The payment requirements
   * @returns Validation response
   */
  async verify(
    payload: PaymentPayload,
    requirements: PaymentRequirements,
  ): Promise<VerifyResponse> {
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload

    // Check time constraints
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
    const asset = requirements.asset || 'EGLD'

    if (asset !== 'EGLD') {
      // ESDT
      if (relayedPayload.receiver !== relayedPayload.sender) {
        return {
          isValid: false,
          invalidReason: `Receiver mismatch for ESDT (expected self-send to ${relayedPayload.sender}, got ${relayedPayload.receiver})`,
        }
      }

      const parts = (relayedPayload.data || '').split('@')
      if (parts.length < 6 || parts[0] !== 'MultiESDTNFTTransfer') {
        return { isValid: false, invalidReason: 'Invalid ESDT transfer data format' }
      }

      const destHex = parts[1]
      const destAddr = new Address(expectedReceiver)
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
      // EGLD
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

    // Step 1: Verify Ed25519 signature to ensure sender authorized the transaction
    const signatureValid = await this.verifySignature(relayedPayload)
    if (!signatureValid.isValid) {
      return signatureValid
    }

    // Step 2: Simulate transaction to validate it will succeed on-chain
    // This provides additional safety by catching issues before broadcasting
    return this.verifyViaSimulation(relayedPayload)
  }

  /**
   * Settles the payment by broadcasting the transaction.
   *
   * @param payload - The payment payload
   * @param requirements - The payment requirements
   * @returns Settle response
   */
  async settle(
    payload: PaymentPayload,
    requirements: PaymentRequirements,
  ): Promise<SettleResponse> {
    const relayedPayload = payload.payload as unknown as ExactMultiversXPayload
    const network = requirements.network as Network
    const payer = relayedPayload.sender

    // Attempt to sign as relayer if credentials provided and signature is missing
    let relayerSig = relayedPayload.relayerSignature
    const relayerAddr = relayedPayload.relayer

    // Attempt relay signing if supported/needed
    if (this.signer && this.signerAddress && relayerAddr === this.signerAddress && !relayerSig) {
      // Relayer signing implementation would go here
    }

    try {
      const txSendBody = {
        nonce: relayedPayload.nonce,
        value: relayedPayload.value,
        receiver: relayedPayload.receiver,
        sender: relayedPayload.sender,
        gasPrice: relayedPayload.gasPrice,
        gasLimit: relayedPayload.gasLimit,
        data: Buffer.from(relayedPayload.data || '').toString('base64'),
        signature: relayedPayload.signature,
        chainID: relayedPayload.chainID,
        version: relayedPayload.version,
        relayer: relayedPayload.relayer,
        relayerSignature: relayerSig,
      }

      const response = await fetch(`${this.apiUrl}/transaction/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(txSendBody),
      })

      if (!response.ok) {
        return {
          success: false,
          errorReason: `Broadcast failed: ${response.statusText}`,
          transaction: '',
          network,
          payer,
        }
      }

      const txResult = await response.json()
      if (txResult.error) {
        return { success: false, errorReason: txResult.error, transaction: '', network, payer }
      }

      const txHash = txResult.data?.txHash

      if (!txHash) {
        return {
          success: false,
          errorReason: 'Broadcast succeeded but no hash returned',
          transaction: '',
          network,
          payer,
        }
      }

      // Parity Feature: Wait for Transaction
      try {
        await this.waitForTx(txHash)
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
   * Verifies the payload via simulation.
   *
   * @param payload - The payload to verify
   * @returns Validation response
   */
  private async verifyViaSimulation(payload: ExactMultiversXPayload): Promise<VerifyResponse> {
    try {
      let relayerSig = payload.relayerSignature
      const relayerAddr = payload.relayer

      if (this.signer && this.signerAddress && relayerAddr === this.signerAddress && !relayerSig) {
        // We are the relayer and need to sign
        const txToSign = new Transaction({
          nonce: BigInt(payload.nonce),
          value: payload.value,
          receiver: new Address(payload.receiver),
          sender: new Address(payload.sender),
          gasLimit: payload.gasLimit,
          gasPrice: BigInt(payload.gasPrice), // SDK expects bigint?
          data: new TransactionPayload(payload.data),
          chainID: payload.chainID,
          version: payload.version,
          options: payload.options,
        })

        // Apply existing signature
        txToSign.applySignature(Buffer.from(payload.signature || '', 'hex'))

        // Set Relayer
        if (relayerAddr) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          ; (txToSign as any).relayer = new Address(relayerAddr)
        }

        relayerSig = await this.signer.signTransaction(txToSign)
      }

      const simulationBody = {
        nonce: payload.nonce,
        value: payload.value,
        receiver: payload.receiver,
        sender: payload.sender,
        gasPrice: payload.gasPrice,
        gasLimit: payload.gasLimit,
        data: Buffer.from(payload.data || '').toString('base64'),
        chainID: payload.chainID,
        version: payload.version,
        signature: payload.signature,
        relayer: payload.relayer,
        relayerSignature: relayerSig, // might be empty
      }

      const response = await fetch(`${this.apiUrl}/transaction/simulate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(simulationBody),
      })

      if (!response.ok) {
        return { isValid: false, invalidReason: `Simulation API error: ${response.status}` }
      }

      const simResult = await response.json()
      if (simResult.error) {
        return { isValid: false, invalidReason: `Simulation error: ${simResult.error}` }
      }
      if (
        simResult.data?.result?.status !== 'success' &&
        simResult.data?.result?.status !== 'successful'
      ) {
        // Check return message
        const failReason = simResult.data?.result?.returnMessage || simResult.data?.result?.status
        return {
          isValid: false,
          invalidReason: `Simulation failed: ${failReason}`,
        }
      }

      return { isValid: true }
    } catch (e: unknown) {
      const err = e as Error
      return { isValid: false, invalidReason: `Simulation exception: ${err.message}` }
    }
  }

  /**
   * Verifies the Ed25519 signature of the transaction (crypto-only, no blockchain call).
   * This matches EVM's signature verification pattern - simulation happens in settle.
   *
   * @param payload - The payload to verify
   * @returns Validation response
   */
  private async verifySignature(payload: ExactMultiversXPayload): Promise<VerifyResponse> {
    try {
      if (!payload.signature) {
        return { isValid: false, invalidReason: 'Missing signature' }
      }

      // Reconstruct the transaction for signature verification
      const tx = new Transaction({
        nonce: BigInt(payload.nonce),
        value: payload.value,
        receiver: new Address(payload.receiver),
        sender: new Address(payload.sender),
        gasLimit: payload.gasLimit,
        gasPrice: BigInt(payload.gasPrice),
        data: new TransactionPayload(payload.data || ''),
        chainID: payload.chainID,
        version: payload.version,
        options: payload.options,
      })

      // Get the message to verify (transaction bytes for signing)
      const { TransactionComputer } = await import('@multiversx/sdk-core')
      const txComputer = new TransactionComputer()
      const serializedTx = txComputer.computeBytesForSigning(tx)

      // Verify signature using @noble/ed25519 (v3 API)
      const ed = await import('@noble/ed25519')

      // Get public key from sender address
      const senderAddress = new Address(payload.sender)
      const publicKeyBytes = senderAddress.getPublicKey()
      const signatureBytes = Buffer.from(payload.signature, 'hex')

      // ed25519 v3 verify returns a Promise<boolean>
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
   * Polls the transaction status until success or failure/timeout.
   *
   * @param txHash - The transaction hash to wait for
   */
  private async waitForTx(txHash: string): Promise<void> {
    const timeoutMs = 120000 // 120 seconds
    const pollIntervalMs = 2000
    const startTime = Date.now()

    while (Date.now() - startTime < timeoutMs) {
      try {
        const response = await fetch(`${this.apiUrl}/transaction/${txHash}/status`)
        if (!response.ok) {
          // Transient error, retry
          await new Promise((r) => setTimeout(r, pollIntervalMs))
          continue
        }

        const res = await response.json()
        const status = res.data?.status || res.status // handle different API response shapes

        if (['success', 'successful', 'executed'].includes(status)) {
          return
        }
        if (['fail', 'failed', 'invalid'].includes(status)) {
          // Fetch error detail
          let errorMsg = `Transaction status: ${status}`
          try {
            const infoRes = await fetch(`${this.apiUrl}/transaction/${txHash}`)
            if (infoRes.ok) {
              await infoRes.json()
              // Try deeper path first (e.g. data.transaction.smartContractResults...) or just top level error
              // Standard API usually puts error in data.transaction.error or execution code
              // We'll stick to simple status for parity unless we parse full info
            }
          } catch { } // best effort
          throw new Error(errorMsg)
        }
        // pending, processing, etc.
        await new Promise((r) => setTimeout(r, pollIntervalMs))
      } catch (e) {
        // If it's the error we threw, rethrow
        if (e instanceof Error && e.message.startsWith('Transaction status')) throw e
        // Otherwise transient network error, retry
        await new Promise((r) => setTimeout(r, pollIntervalMs))
      }
    }
    throw new Error(`Timeout waiting for tx ${txHash}`)
  }
}
