import { PaymentPayload, PaymentRequirements, SchemeNetworkClient } from '@x402/core/types'
import { MultiversXSigner } from '../../signer'
import { ExactMultiversXPayload } from '../../types'
import { ApiNetworkProvider } from '@multiversx/sdk-network-providers'
import { Address, Transaction, TransactionPayload } from '@multiversx/sdk-core'
import {
  CHAIN_ID_DEVNET,
  CHAIN_ID_MAINNET,
  CHAIN_ID_TESTNET,
  MULTIVERSX_API_URL_DEVNET,
  MULTIVERSX_API_URL_MAINNET,
  MULTIVERSX_API_URL_TESTNET,
  MULTIVERSX_GAS_BASE_COST,
  MULTIVERSX_GAS_MULTI_TRANSFER_COST,
  MULTIVERSX_GAS_PER_BYTE,
  MULTIVERSX_GAS_PRICE_DEFAULT,
  MULTIVERSX_GAS_RELAYED_COST,
  MULTIVERSX_TRANSFER_METHOD_DIRECT,
} from '../../constants'

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
    const chainRef = networkParts.length > 1 ? networkParts[1] : CHAIN_ID_MAINNET
    let apiUrl = MULTIVERSX_API_URL_MAINNET
    if (chainRef === CHAIN_ID_DEVNET) apiUrl = MULTIVERSX_API_URL_DEVNET
    if (chainRef === CHAIN_ID_TESTNET) apiUrl = MULTIVERSX_API_URL_TESTNET

    const provider = new ApiNetworkProvider(apiUrl)
    const senderAddressStr = await this.signer.getAddress()

    const senderAddress = new Address(senderAddressStr)

    let nonce = 0
    try {
      const accountData = await provider.getAccount(senderAddress)
      nonce = accountData.nonce
    } catch (error) {
      console.warn('Failed to fetch account for nonce, defaulting to 0', error)
    }

    const transferMethod = paymentRequirements.extra?.assetTransferMethod as string
    let version = 2
    if (transferMethod === MULTIVERSX_TRANSFER_METHOD_DIRECT) {
      version = 1
    }

    const relayer = paymentRequirements.extra?.relayer as string
    if (version !== 1 && !relayer) {
      throw new Error('relayer address is required for relayed transfers')
    }

    const { dataString, receiver, value } = this.constructTransferData(
      paymentRequirements,
      senderAddressStr,
    )
    const gasLimit = this.calculateGasLimit(paymentRequirements, dataString)
    const gasPrice = MULTIVERSX_GAS_PRICE_DEFAULT

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
      version: version,
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
      version,
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

  /**
   * Calculates the gas limit for a MultiversX transaction.
   *
   * @param requirements - Payment requirements potentially containing manual gasLimit
   * @param dataString - The transaction data string
   * @returns The calculated gas limit
   */
  private calculateGasLimit(requirements: PaymentRequirements, dataString: string): number {
    if (requirements.extra?.gasLimit) {
      const gl = requirements.extra.gasLimit
      if (typeof gl === 'number') return gl
      if (typeof gl === 'string') return parseInt(gl, 10)
    }

    const asset = requirements.asset

    // Base gas limit calculation mirrored from Go utils.CalculateGasLimit
    // numTransfers is strictly 1 for this flow
    const dataBytes = Buffer.from(dataString, 'utf8')
    let gasLimit =
      MULTIVERSX_GAS_BASE_COST +
      MULTIVERSX_GAS_PER_BYTE * dataBytes.length +
      MULTIVERSX_GAS_MULTI_TRANSFER_COST + // per 1 transfer
      MULTIVERSX_GAS_RELAYED_COST

    const scFunction = requirements.extra?.scFunction as string
    const isScCall = !!scFunction || asset !== 'EGLD'

    if (isScCall) {
      gasLimit += 10_000_000
    }

    return gasLimit
  }

  /**
   * Constructs the MultiversX transaction data and resolves destination/value.
   *
   * @param requirements - The payment requirements
   * @param senderAddressStr - The sender's bech32 address
   * @returns The data string, receiver address, and value string
   */
  private constructTransferData(
    requirements: PaymentRequirements,
    senderAddressStr: string,
  ): { dataString: string; receiver: string; value: string } {
    const asset = requirements.asset
    const scFunction = requirements.extra?.scFunction as string
    const args: string[] = []
    if (Array.isArray(requirements.extra?.arguments)) {
      requirements.extra.arguments.forEach((arg) => {
        if (typeof arg === 'string') args.push(arg)
      })
    }

    if (asset && asset !== 'EGLD') {
      const destAddress = new Address(requirements.payTo)
      const destHex = destAddress.hex()
      const tokenHex = Buffer.from(asset, 'utf8').toString('hex')

      let amountBi = BigInt(requirements.amount)
      let amountHex = amountBi.toString(16)
      if (amountHex.length % 2 !== 0) amountHex = '0' + amountHex

      const parts = ['MultiESDTNFTTransfer', destHex, '01', tokenHex, '00', amountHex]

      if (scFunction) {
        parts.push(Buffer.from(scFunction, 'utf8').toString('hex'))
        if (args.length > 0) {
          parts.push(...args)
        }
      }

      return {
        dataString: parts.join('@'),
        receiver: senderAddressStr,
        value: '0',
      }
    }

    const parts: string[] = []
    if (scFunction) {
      parts.push(scFunction)
      if (args.length > 0) {
        parts.push(...args)
      }
    }

    return {
      dataString: parts.join('@'),
      receiver: requirements.payTo,
      value: requirements.amount,
    }
  }
}
