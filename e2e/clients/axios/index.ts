import { config } from "dotenv";
import axios from "axios";
import { wrapAxiosWithPayment, decodePaymentResponseHeader } from "@x402/axios";
import { privateKeyToAccount } from "viem/accounts";
import { registerExactEvmScheme } from "@x402/evm/exact/client";
import { registerExactSvmScheme } from "@x402/svm/exact/client";
import { registerExactMultiversXClientScheme } from "@x402/multiversx/exact/client";
import { MultiversXSigner, ISignerProvider } from "@x402/multiversx";
import { UserSigner, UserSecretKey } from "@multiversx/sdk-wallet";
import { Transaction } from "@multiversx/sdk-core";
import { base58 } from "@scure/base";
import { createKeyPairSignerFromBytes } from "@solana/kit";
import { x402Client } from "@x402/core/client";

config();

const baseURL = process.env.RESOURCE_SERVER_URL as string;
const endpointPath = process.env.ENDPOINT_PATH as string;
const url = `${baseURL}${endpointPath}`;
const evmAccount = privateKeyToAccount(process.env.EVM_PRIVATE_KEY as `0x${string}`);
const svmSigner = await createKeyPairSignerFromBytes(
  base58.decode(process.env.SVM_PRIVATE_KEY as string),
);

// Create client and register EVM and SVM schemes using the new register helpers
const client = new x402Client();
registerExactEvmScheme(client, { signer: evmAccount });
registerExactSvmScheme(client, { signer: svmSigner });

// Adapter to wrap UserSigner to ISignerProvider interface
/**
 * Adapter class that wraps UserSigner to implement ISignerProvider interface.
 */
class UserSignerAdapter implements ISignerProvider {
  /**
   * Creates a new UserSignerAdapter.
   *
   * @param userSigner - The underlying UserSigner instance
   * @param address - The bech32 address of the signer
   */
  constructor(
    private userSigner: UserSigner,
    private address: string,
  ) {}

  /**
   * Signs a transaction using the underlying UserSigner.
   *
   * @param transaction - The transaction to sign
   * @returns The signed transaction
   */
  async signTransaction(transaction: Transaction): Promise<Transaction> {
    const serialized = transaction.serializeForSigning();
    const signature = await this.userSigner.sign(serialized);
    transaction.applySignature(signature);
    return transaction;
  }

  /**
   * Gets the address of the signer.
   *
   * @returns The bech32 address
   */
  async getAddress(): Promise<string> {
    return this.address;
  }
}

// Register MultiversX if key is provided
const mvxPrivateKeyHex = process.env.MVX_PRIVATE_KEY;
if (mvxPrivateKeyHex && mvxPrivateKeyHex.length === 64) {
  try {
    const secretKey = new UserSecretKey(Buffer.from(mvxPrivateKeyHex, "hex"));
    const userSigner = new UserSigner(secretKey);
    const address = secretKey.generatePublicKey().toAddress().bech32();
    const signerAdapter = new UserSignerAdapter(userSigner, address);
    const mvxSigner = new MultiversXSigner(signerAdapter);
    registerExactMultiversXClientScheme(client, { signer: mvxSigner });
  } catch {
    console.error("⚠️ Failed to load MultiversX private key");
  }
}

const axiosWithPayment = wrapAxiosWithPayment(axios.create(), client);

axiosWithPayment
  .get(url)
  .then(async response => {
    const data = response.data;
    // Check both v2 (PAYMENT-RESPONSE) and v1 (X-PAYMENT-RESPONSE) headers
    const paymentResponse =
      response.headers["payment-response"] || response.headers["x-payment-response"];

    if (!paymentResponse) {
      // No payment was required
      const result = {
        success: true,
        data: data,
        status_code: response.status,
      };
      console.log(JSON.stringify(result));
      process.exit(0);
      return;
    }

    const decodedPaymentResponse = decodePaymentResponseHeader(paymentResponse);

    const result = {
      success: decodedPaymentResponse.success,
      data: data,
      status_code: response.status,
      payment_response: decodedPaymentResponse,
    };

    // Output structured result as JSON for proxy to parse
    console.log(JSON.stringify(result));
    process.exit(0);
  })
  .catch(error => {
    console.error(
      JSON.stringify({
        success: false,
        error: error.message || "Request failed",
        status_code: error.response?.status || 500,
      }),
    );
    process.exit(1);
  });
