import {
    Transaction,
    Address,
    TokenPayment,
    TransactionPayload,
    TokenTransfer
} from '@multiversx/sdk-core';
import { MULTIVERSX_GAS_LIMIT_EGLD, MULTIVERSX_GAS_LIMIT_ESDT } from './constants';
import { ExactMultiversXAuthorization } from './types';

// Interface for the Wallet Provider (e.g., Extension, Ledger)
export interface ISignerProvider {
    signTransaction(transaction: Transaction): Promise<Transaction>;
    getAddress(): Promise<string>;
}

export class MultiversXSigner {
    constructor(
        private provider: ISignerProvider,
        public address: string // Publicly accessible address
    ) { }

    /**
     * Signs a x402 payment authorization (Transaction).
     * Handles both EGLD (Direct) and ESDT (Transfer & Execute) payments.
     */
    async signTransaction(auth: ExactMultiversXAuthorization, chainId: string): Promise<string> {
        let transaction: Transaction;

        // 1. Prepare Function Call: pay@<resource_id_hex>
        const resourceIdBuff = Buffer.from(auth.resourceId, 'utf8');
        const resourceIdHex = resourceIdBuff.toString('hex');
        const payHex = Buffer.from("pay", 'utf8').toString('hex');

        // Logic split: EGLD vs ESDT
        if (auth.tokenIdentifier === 'EGLD') {
            // Case A: Direct EGLD Payment
            const data = new TransactionPayload(`pay@${resourceIdHex}`);
            const value = TokenTransfer.egldFromBigInteger(auth.value); // auth.value is atomic units string

            transaction = new Transaction({
                nonce: auth.nonce ? BigInt(auth.nonce) : undefined,
                value: value,
                receiver: new Address(auth.to),
                sender: new Address(this.address),
                gasLimit: MULTIVERSX_GAS_LIMIT_EGLD,
                data: data,
                chainID: chainId
            });

        } else {
            // Case B: ESDT Payment
            // multiESDTNFTTransfer
            const receiver = new Address(auth.to);
            const tokenHex = Buffer.from(auth.tokenIdentifier, 'utf8').toString('hex');

            // Amount handling: simple parse for Atomic Units (assuming input is atomic/integer string)
            // If input is "1.5", we'd need decimals. Protocol usually deals in atomic units.
            let amountBi = BigInt(auth.value);
            let amountHex = amountBi.toString(16);
            if (amountHex.length % 2 !== 0) amountHex = "0" + amountHex;

            // Data: MultiESDTNFTTransfer@<dest_hex>@01@<token_hex>@00@<amount_hex>@pay@<resource_id_hex>
            const dataString = `MultiESDTNFTTransfer@${receiver.hex()}@01@${tokenHex}@00@${amountHex}@${payHex}@${resourceIdHex}`;

            transaction = new Transaction({
                nonce: auth.nonce ? BigInt(auth.nonce) : undefined,
                value: TokenTransfer.egldFromAmount("0"),
                receiver: new Address(this.address), // Send to self
                sender: new Address(this.address),
                gasLimit: MULTIVERSX_GAS_LIMIT_ESDT,
                data: new TransactionPayload(dataString),
                chainID: chainId
            });
        }

        // 2. Sign
        const signedTx = await this.provider.signTransaction(transaction);

        // 3. Return Hash
        return signedTx.getHash().toString();
    }
}
