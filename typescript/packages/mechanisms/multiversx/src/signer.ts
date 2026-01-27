import { Transaction, Address, TokenTransfer, TransactionPayload } from '@multiversx/sdk-core'

// Interface matching the SDK's signing provider
export interface ISignerProvider {
  signTransaction(transaction: Transaction): Promise<Transaction>
  getAddress?(): Promise<string>
}

export interface PaymentRequest {
  to: string
  amount: string
  tokenIdentifier: string
  resourceId: string
  chainId: string
  nonce?: number
}

/**
 * MultiversX Signer implementation.
 */
export class MultiversXSigner {
  /**
   * Creates a new MultiversX Signer.
   *
   * @param provider - The signing provider
   * @param senderAddress - Optional explicit sender address
   */
  constructor(
    private provider: ISignerProvider,
    private senderAddress?: string,
  ) {}

  /**
   * Signs a x402 payment transaction.
   *
   * @param request - The payment request details
   * @returns The signature hex string
   */
  async sign(request: PaymentRequest): Promise<string> {
    const sender = await this.getSender()
    let transaction: Transaction

    // EGLD Payment
    if (request.tokenIdentifier === 'EGLD') {
      const value = TokenTransfer.egldFromBigInteger(request.amount)

      // Encode resourceId in data field
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
      // ESDT Payment
      // ESDT Payment
      // Use "MultiESDTNFTTransfer" to send tokens

      const resourceIdHex = Buffer.from(request.resourceId, 'utf8').toString('hex')
      const tokenHex = Buffer.from(request.tokenIdentifier, 'utf8').toString('hex')

      // Destination Address to Hex
      const destAddress = new Address(request.to)
      const destHex = destAddress.hex()

      // Handle Amount
      let amountBi = BigInt(request.amount)
      let amountHex = amountBi.toString(16)
      if (amountHex.length % 2 !== 0) amountHex = '0' + amountHex

      // Data: MultiESDTNFTTransfer @ <DestHex> @ <NumTransfers(01)> @ <TokenHex> @ <Nonce(00)> @ <AmountHex> @ <ResourceID>
      const dataString = `MultiESDTNFTTransfer@${destHex}@01@${tokenHex}@00@${amountHex}@${resourceIdHex}`

      transaction = new Transaction({
        nonce: request.nonce ? BigInt(request.nonce) : undefined,
        value: TokenTransfer.egldFromAmount('0'),
        receiver: new Address(sender), // Send to Self
        sender: new Address(sender),
        gasLimit: 60_000_000, // Higher gas for MultiESDT
        data: new TransactionPayload(dataString),
        chainID: request.chainId,
      })
    }

    const signedTx = await this.provider.signTransaction(transaction)
    return signedTx.getSignature().toString('hex')
  }

  /**
   * Signs a pre-constructed transaction.
   *
   * @param transaction - The transaction object
   * @returns The signature hex string
   */
  async signTransaction(transaction: Transaction): Promise<string> {
    const signedTx = await this.provider.signTransaction(transaction)
    return signedTx.getSignature().toString('hex')
  }

  /**
   * Retrieves the sender address from explicit config or provider.
   *
   * @returns The sender address
   */
  async getAddress(): Promise<string> {
    return this.getSender()
  }

  /**
   * Retrieves the sender address from explicit config or provider (internal).
   *
   * @returns The sender address
   */
  private async getSender(): Promise<string> {
    if (this.senderAddress) return this.senderAddress
    if (this.provider.getAddress) return await this.provider.getAddress()
    throw new Error('Sender address not provided and provider does not support getAddress')
  }
}
