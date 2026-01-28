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
    private apiUrl: string = 'https://devnet-api.multiversx.com',
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
    const asset = requirements.asset || 'EGLD'

    if (asset !== 'EGLD') {
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

    return this.verifyViaSimulation(relayedPayload)
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
    let relayerSig = relayedPayload.relayerSignature

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
   * Performs an on-chain simulation of the transaction to verify its validity.
   *
   * @param payload - The payload to simulate
   * @returns A verification response based on simulation results
   */
  private async verifyViaSimulation(payload: ExactMultiversXPayload): Promise<VerifyResponse> {
    try {
      let relayerSig = payload.relayerSignature
      const relayerAddr = payload.relayer

      if (this.signer && this.signerAddress && relayerAddr === this.signerAddress && !relayerSig) {
        const txToSign = new Transaction({
          nonce: BigInt(payload.nonce),
          value: payload.value,
          receiver: new Address(payload.receiver),
          sender: new Address(payload.sender),
          gasLimit: payload.gasLimit,
          gasPrice: BigInt(payload.gasPrice),
          data: new TransactionPayload(payload.data),
          chainID: payload.chainID,
          version: payload.version,
          options: payload.options,
        })

        txToSign.applySignature(Buffer.from(payload.signature || '', 'hex'))

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
        relayerSignature: relayerSig,
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
        receiver: new Address(payload.receiver),
        sender: new Address(payload.sender),
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

      const senderAddress = new Address(payload.sender)
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
   * Status check helper that polls the MultiversX API until a transaction is finalized or fails.
   *
   * @param txHash - The hash of the transaction to poll for
   * @throws Error if the transaction fails or timeouts
   */
  private async waitForTx(txHash: string): Promise<void> {
    const timeoutMs = 120000
    const pollIntervalMs = 2000
    const startTime = Date.now()

    while (Date.now() - startTime < timeoutMs) {
      try {
        const response = await fetch(`${this.apiUrl}/transaction/${txHash}/status`)
        if (!response.ok) {
          await new Promise((r) => setTimeout(r, pollIntervalMs))
          continue
        }

        const res = await response.json()
        const status = res.data?.status || res.status

        if (['success', 'successful', 'executed'].includes(status)) {
          return
        }
        if (['fail', 'failed', 'invalid'].includes(status)) {
          let errorMsg = `Transaction status: ${status}`
          try {
            await fetch(`${this.apiUrl}/transaction/${txHash}`)
          } catch { }
          throw new Error(errorMsg)
        }
        await new Promise((r) => setTimeout(r, pollIntervalMs))
      } catch (e) {
        if (e instanceof Error && e.message.startsWith('Transaction status')) throw e
        await new Promise((r) => setTimeout(r, pollIntervalMs))
      }
    }
    throw new Error(`Timeout waiting for tx ${txHash}`)
  }
}
