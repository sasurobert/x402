import { Transaction, Address, TokenTransfer, TransactionPayload } from '@multiversx/sdk-core'

/**
 * Provider interface for signing MultiversX transactions.
 */
export interface ISignerProvider {
  /**
   * Signs a MultiversX transaction.
   *
   * @param transaction - The transaction to sign
   * @returns A promise that resolves to the signed transaction
   */
  signTransaction(transaction: Transaction): Promise<Transaction>

  /**
   * Gets the address of the signer.
   *
   * @returns A promise that resolves to the bech32 address string
   */
  getAddress?(): Promise<string>
}

/**
 * Standard payment request structure for the MultiversX signer.
 */
export interface PaymentRequest {
  /** The recipient bech32 address */
  to: string
  /** The amount in atomic units as a string */
  amount: string
  /** The token identifier (e.g., 'EGLD' or an ESDT ID) */
  tokenIdentifier: string
  /** The resource ID to be included in the transaction data */
  resourceId: string
  /** The network chain ID */
  chainId: string
  /** Optional nonce for the transaction */
  nonce?: number
}

/**
 * MultiversX Signer implementation for creating and signing standard payment transactions.
 */
export class MultiversXSigner {
  /**
   * Initializes the MultiversXSigner.
   *
   * @param provider - The signing provider (e.g., an extension or hardware wallet)
   * @param senderAddress - Optional sender address if not provided by the provider
   */
  constructor(
    private provider: ISignerProvider,
    private senderAddress?: string,
  ) { }

  /**
   * Signs a high-level payment request.
   *
   * @param request - The payment request details
   * @returns The hexadecimal signature string
   */
  async sign(request: PaymentRequest): Promise<string> {
    const sender = await this.getSender()
    let transaction: Transaction

    if (request.tokenIdentifier === 'EGLD') {
      const value = TokenTransfer.egldFromBigInteger(request.amount)
      const data = new TransactionPayload(request.resourceId)

      transaction = new Transaction({
        nonce: request.nonce ? BigInt(request.nonce) : undefined,
        value: value,
        receiver: new Address(request.to),
        sender: new Address(sender),
        gasLimit: 50_000,
        data: data,
        chainID: request.chainId,
      })
    } else {
      const resourceIdHex = Buffer.from(request.resourceId, 'utf8').toString('hex')
      const tokenHex = Buffer.from(request.tokenIdentifier, 'utf8').toString('hex')
      const destAddress = new Address(request.to)
      const destHex = destAddress.hex()

      let amountBi = BigInt(request.amount)
      let amountHex = amountBi.toString(16)
      if (amountHex.length % 2 !== 0) amountHex = '0' + amountHex

      const dataString = `MultiESDTNFTTransfer@${destHex}@01@${tokenHex}@00@${amountHex}@${resourceIdHex}`

      transaction = new Transaction({
        nonce: request.nonce ? BigInt(request.nonce) : undefined,
        value: TokenTransfer.egldFromAmount('0'),
        receiver: new Address(sender),
        sender: new Address(sender),
        gasLimit: 60_000_000,
        data: new TransactionPayload(dataString),
        chainID: request.chainId,
      })
    }

    const signedTx = await this.provider.signTransaction(transaction)
    return signedTx.getSignature().toString('hex')
  }

  /**
   * Signs a raw MultiversX transaction.
   *
   * @param transaction - The transaction object to sign
   * @returns The hexadecimal signature string
   */
  async signTransaction(transaction: Transaction): Promise<string> {
    const signedTx = await this.provider.signTransaction(transaction)
    return signedTx.getSignature().toString('hex')
  }

  /**
   * Gets the address of the signer.
   *
   * @returns The bech32 address string
   */
  async getAddress(): Promise<string> {
    return this.getSender()
  }

  /**
   * Internal helper to resolve the sender address.
   *
   * @returns The resolved bech32 address string
   * @throws Error if address cannot be resolved
   */
  private async getSender(): Promise<string> {
    if (this.senderAddress) return this.senderAddress
    if (this.provider.getAddress) return await this.provider.getAddress()
    throw new Error('Sender address not provided and provider does not support getAddress')
  }
}
