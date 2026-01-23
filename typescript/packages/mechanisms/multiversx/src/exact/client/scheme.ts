import { PaymentPayload, PaymentRequirements, SchemeNetworkClient } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXPayload, ExactMultiversXAuthorization } from '../../types'
import { ApiNetworkProvider } from '@multiversx/sdk-network-providers'
import { Account, Address } from '@multiversx/sdk-core'

/**
 * MultiversX client implementation for the Exact payment scheme.
 */
export class ExactMultiversXScheme implements SchemeNetworkClient {
  readonly scheme = 'exact'

  /**
   * Creates a new Exact MultiversX Scheme client.
   *
   * @param signer - The MultiversX signer instance
   */
  constructor(private readonly signer: MultiversXSigner) { }

  /**
   * Creates a payment payload.
   *
   * @param x402Version - The protocol version
   * @param paymentRequirements - The payment requirements
   * @returns The payment payload wrapper
   */
  async createPaymentPayload(
    x402Version: number,
    paymentRequirements: PaymentRequirements,
  ): Promise<Pick<PaymentPayload, 'x402Version' | 'payload'>> {
    const now = Math.floor(Date.now() / 1000)

    // Parse ChainID and Helper for API URL
    const networkParts = paymentRequirements.network.split(':')
    const chainRef = networkParts.length > 1 ? networkParts[1] : '1'
    let apiUrl = 'https://api.multiversx.com'
    if (chainRef === 'D') apiUrl = 'https://devnet-api.multiversx.com'
    if (chainRef === 'T') apiUrl = 'https://testnet-api.multiversx.com'

    // Fetch Nonce from Network
    const provider = new ApiNetworkProvider(apiUrl)
    const senderAddress = new Address(this.signer.address)

    let nonce = 0
    try {
      const accountData = await provider.getAccount(senderAddress)
      nonce = accountData.nonce
    } catch (error) {
      console.warn('Failed to fetch account for nonce, defaulting to 0', error)
    }

    // We assume 'paymentRequirements.asset' holds the Token Identifier (EGLD or TokenID)
    // The 'payTo' is the SC Address.
    // The 'extra' field contains resourceId.

    const resourceId = paymentRequirements.extra?.resourceId
    if (typeof resourceId !== 'string' || !resourceId) {
      throw new Error(
        'resourceId is required and must be a string in payment requirements extra field',
      )
    }

    const authorization: ExactMultiversXAuthorization = {
      from: this.signer.address,
      to: paymentRequirements.payTo,
      value: paymentRequirements.amount,
      tokenIdentifier: paymentRequirements.asset, // asset field used as TokenID
      resourceId: resourceId,
      validAfter: (now - 600).toString(), // 10 minutes ago
      validBefore: (now + paymentRequirements.maxTimeoutSeconds).toString(),
      nonce: nonce,
    }

    const chainId = chainRef

    const request = {
      to: authorization.to,
      amount: authorization.value,
      tokenIdentifier: authorization.tokenIdentifier,
      resourceId: authorization.resourceId,
      chainId: chainId,
      nonce: authorization.nonce,
    }

    const signatureHex = await this.signer.sign(request)

    // IMPORTANT: The payload nonce MUST match the signed nonce.
    // The previous implementation had a placeholder 0.
    // Now we use the fetched nonce.

    const payload: ExactMultiversXPayload = {
      nonce: authorization.nonce!,
      value: authorization.value,
      receiver: authorization.to,
      sender: authorization.from,
      gasPrice: 1000000000,
      gasLimit: authorization.tokenIdentifier === 'EGLD' ? 50000 : 60000000,
      data: '', // Computed/Verified by Facilitator, but we could populate it if we wanted strict parity
      chainID: chainId,
      version: 1,
      options: 0,
      signature: signatureHex,
      authorization, // Optional context
    }

    // Populate data field for completeness/verification matching
    if (authorization.tokenIdentifier && authorization.tokenIdentifier !== 'EGLD') {
      const resourceIdHex = Buffer.from(resourceId, 'utf8').toString('hex')
      const tokenHex = Buffer.from(authorization.tokenIdentifier, 'utf8').toString('hex')
      const destAddress = new Address(authorization.to)
      const destHex = destAddress.hex()
      let amountBi = BigInt(authorization.value)
      let amountHex = amountBi.toString(16)
      if (amountHex.length % 2 !== 0) amountHex = '0' + amountHex

      // MultiESDTNFTTransfer@<DestHex>@01@<TokenHex>@00@<AmountHex>@<ResourceID>
      const dataString = `MultiESDTNFTTransfer@${destHex}@01@${tokenHex}@00@${amountHex}@${resourceIdHex}`
      payload.data = Buffer.from(dataString).toString('base64')
    } else {
      // EGLD
      // In signer we did: new TransactionPayload(request.resourceId)
      // Which is just the string bytes
      payload.data = Buffer.from(authorization.resourceId).toString('base64')
    }

    return {
      x402Version,
      payload,
    }
  }
}
