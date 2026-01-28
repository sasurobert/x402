import { PaymentPayload, PaymentRequirements, SchemeNetworkClient } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXPayload } from '../../types'
import { ApiNetworkProvider } from '@multiversx/sdk-network-providers'
import { Address, Transaction, TransactionPayload } from '@multiversx/sdk-core'

/**
 * MultiversX Client implementation for the Exact payment scheme.
 */
export class ExactMultiversXScheme implements SchemeNetworkClient {
  /**
   * The scheme identifier for this client.
   */
  readonly scheme = 'exact'

  /**
   * Initializes the ExactMultiversXScheme client.
   *
   * @param signer - The MultiversX signer to use for transaction creation and signing
   */
  constructor(private readonly signer: MultiversXSigner) { }

  /**
   * Creates a payment payload for MultiversX by constructing and signing a transaction.
   *
   * @param x402Version - The version of the x402 protocol being used
   * @param paymentRequirements - The requirements for the payment to be made
   * @returns A partial PaymentPayload containing the version and the MultiversX-specific payload
   */
  async createPaymentPayload(
    x402Version: number,
    paymentRequirements: PaymentRequirements,
  ): Promise<Pick<PaymentPayload, 'x402Version' | 'payload'>> {
    if (!paymentRequirements.payTo) {
      throw new Error('PayTo is required')
    }

    const now = Math.floor(Date.now() / 1000)

    const networkParts = paymentRequirements.network.split(':')
    const chainRef = networkParts.length > 1 ? networkParts[1] : '1'
    let apiUrl = 'https://api.multiversx.com'
    if (chainRef === 'D') apiUrl = 'https://devnet-api.multiversx.com'
    if (chainRef === 'T') apiUrl = 'https://testnet-api.multiversx.com'

    const provider = new ApiNetworkProvider(apiUrl)
    let senderAddressStr: string
    if (this.signer['senderAddress']) {
      senderAddressStr = this.signer['senderAddress']
    } else {
      senderAddressStr = await this.signer.getAddress()
    }

    const senderAddress = new Address(senderAddressStr)

    let nonce = 0
    try {
      const accountData = await provider.getAccount(senderAddress)
      nonce = accountData.nonce
    } catch (error) {
      console.warn('Failed to fetch account for nonce, defaulting to 0', error)
    }

    let gasLimit = 50_000
    if (paymentRequirements.extra?.gasLimit) {
      const gl = paymentRequirements.extra.gasLimit
      if (typeof gl === 'number') gasLimit = gl
      else if (typeof gl === 'string') gasLimit = parseInt(gl, 10)
    }

    const scFunction =
      typeof paymentRequirements.extra?.scFunction === 'string'
        ? paymentRequirements.extra.scFunction
        : undefined

    const args: string[] = []
    if (Array.isArray(paymentRequirements.extra?.arguments)) {
      paymentRequirements.extra.arguments.forEach((arg) => {
        if (typeof arg === 'string') args.push(arg)
      })
    }

    const relayer =
      typeof paymentRequirements.extra?.relayer === 'string'
        ? paymentRequirements.extra.relayer
        : undefined

    const asset = paymentRequirements.asset
    if (!asset) {
      throw new Error('asset is required')
    }

    let receiver = paymentRequirements.payTo
    let value = paymentRequirements.amount
    let dataString = ''
    let gasPrice = 1_000_000_000

    if (asset !== 'EGLD') {
      receiver = senderAddressStr
      value = '0'
      gasLimit = 60_000_000

      const destAddress = new Address(paymentRequirements.payTo)
      const destHex = destAddress.hex()
      const tokenHex = Buffer.from(asset, 'utf8').toString('hex')

      let amountBi = BigInt(paymentRequirements.amount)
      let amountHex = amountBi.toString(16)
      if (amountHex.length % 2 !== 0) amountHex = '0' + amountHex

      const parts = ['MultiESDTNFTTransfer', destHex, '01', tokenHex, '00', amountHex]

      if (scFunction) {
        parts.push(Buffer.from(scFunction, 'utf8').toString('hex'))
      }

      if (args.length > 0) {
        parts.push(...args)
      }

      dataString = parts.join('@')
    } else {
      const parts: string[] = []
      if (scFunction) {
        parts.push(scFunction)
      }
      if (args.length > 0) {
        parts.push(...args)
      }
      if (parts.length > 0) {
        dataString = parts.join('@')
      }
    }

    const validAfter = now - 600
    let validBefore = now + 600
    if (paymentRequirements.maxTimeoutSeconds && paymentRequirements.maxTimeoutSeconds > 0) {
      validBefore = now + paymentRequirements.maxTimeoutSeconds
    }

    const transaction = new Transaction({
      nonce: BigInt(nonce),
      value: value,
      receiver: new Address(receiver),
      sender: senderAddress,
      gasLimit: gasLimit,
      gasPrice: BigInt(gasPrice),
      data: new TransactionPayload(dataString),
      chainID: chainRef,
      version: 2,
    })

    const signature = await this.signer.signTransaction(transaction)

    const payload: ExactMultiversXPayload = {
      nonce,
      value,
      receiver,
      sender: senderAddressStr,
      gasPrice,
      gasLimit,
      data: dataString,
      chainID: chainRef,
      version: 2,
      options: 0,
      signature,
      relayer,
      validAfter,
      validBefore,
    }

    return {
      x402Version,
      payload,
    }
  }
}
