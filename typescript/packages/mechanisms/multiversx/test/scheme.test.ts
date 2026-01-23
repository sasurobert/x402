import { describe, it, expect, vi } from 'vitest'
import { ExactMultiversXScheme } from '../src/exact/client/scheme'
import { MultiversXSigner } from '../src/signer'
import { PaymentRequirements } from '@x402/core/types'

// Mock Signer
import { Transaction, Address } from '@multiversx/sdk-core'

const alice = new Address(Buffer.alloc(32, 1)).bech32()
const bob = new Address(Buffer.alloc(32, 2)).bech32()

const mockTransaction = new Transaction({
  nonce: 7,
  value: '1000',
  receiver: new Address(bob),
  sender: new Address(alice),
  gasLimit: 60000,
  chainID: '1',
})
mockTransaction.applySignature(Buffer.from('mock_sig_hex', 'hex')) // Ensure it has a signature

const mockSigner = {
  address: alice,
  sign: vi.fn(async () => 'mock_sig_hex'),
} as unknown as MultiversXSigner

describe('ExactMultiversXScheme', () => {
  const scheme = new ExactMultiversXScheme(mockSigner)

  it('should create a valid payment payload', async () => {
    // Mock Provider return value
    vi.mock('@multiversx/sdk-network-providers', () => ({
      ApiNetworkProvider: vi.fn().mockImplementation(() => ({
        getAccount: vi.fn().mockResolvedValue({ nonce: 7 }),
      })),
    }))

    const req: PaymentRequirements = {
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:1',
      maxTimeoutSeconds: 300,
      extra: {
        resourceId: 'res-1',
      },
    }

    const result = await scheme.createPaymentPayload(1, req)

    expect(result.x402Version).toBe(1)
    // Relayed Payload checks
    expect(result.payload.signature).toBe('mock_sig_hex')
    expect(result.payload.nonce).toBe(7)
    expect(result.payload.value).toBe('1000')
    expect(result.payload.sender).toBe(alice)
    // Authorization context check
    expect(result.payload.authorization.resourceId).toBe('res-1')

    // Check auto-calculated fields
    const now = Math.floor(Date.now() / 1000)
    expect(Number(result.payload.authorization.validBefore)).toBeGreaterThan(now)
  })

  it('should throw if resourceId is missing', async () => {
    const req: PaymentRequirements = {
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:1',
      maxTimeoutSeconds: 300,
      // missing extra
    }
    await expect(scheme.createPaymentPayload(1, req)).rejects.toThrow('resourceId is required')
  })
})
