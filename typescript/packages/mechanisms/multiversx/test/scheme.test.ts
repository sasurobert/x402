import { describe, it, expect, vi } from 'vitest'
import { ExactMultiversXScheme } from '../src/exact/client/scheme'
import { MultiversXSigner } from '../src/signer'
import { PaymentRequirements } from '@x402/core/types'
import { Address } from '@multiversx/sdk-core'
import { ExactMultiversXPayload } from '../src/types'

const alice = new Address(Buffer.alloc(32, 1)).bech32()
const bob = new Address(Buffer.alloc(32, 2)).bech32()
const relayer = new Address(Buffer.alloc(32, 3)).bech32()

const mockSigner = {
  getAddress: vi.fn(async () => alice),
  signTransaction: vi.fn(async (_tx) => 'mock_sig_hex'),
} as unknown as MultiversXSigner

describe('ExactMultiversXScheme', () => {
  const scheme = new ExactMultiversXScheme(mockSigner)

  it('should create a valid payment payload', async () => {
    vi.mock('@multiversx/sdk-network-providers', () => ({
      ApiNetworkProvider: vi.fn().mockImplementation(() => ({
        getAccount: vi.fn().mockResolvedValue({ nonce: 7 }),
      })),
    }))

    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 300,
      extra: {
        resourceId: 'res-1',
        relayer: relayer,
      },
    }

    const { x402Version, payload } = await scheme.createPaymentPayload(1, req)
    const exactPayload = payload as ExactMultiversXPayload

    expect(x402Version).toBe(1)
    expect(exactPayload.signature).toBe('mock_sig_hex')
    expect(exactPayload.nonce).toBe(7)
    expect(exactPayload.value).toBe('1000')
    expect(exactPayload.sender).toBe(alice)
    expect(exactPayload.receiver).toBe(bob)
    expect(exactPayload.chainID).toBe('D')
    expect(exactPayload.version).toBe(2)
    expect(exactPayload.gasPrice).toBe(1_000_000_000)
    expect(exactPayload.relayer).toBe(relayer)

    const now = Math.floor(Date.now() / 1000)
    expect(exactPayload.validBefore).toBeGreaterThan(now)
    expect(exactPayload.validAfter).toBeLessThanOrEqual(now)
  })

  it('should create a valid direct EGLD payload (version 1)', async () => {
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '100',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {
        assetTransferMethod: 'direct',
      },
    }

    const { payload } = await scheme.createPaymentPayload(1, req)
    const exactPayload = payload as ExactMultiversXPayload

    expect(exactPayload.version).toBe(1)
    expect(exactPayload.relayer).toBeUndefined()
    expect(exactPayload.receiver).toBe(bob)
    expect(exactPayload.value).toBe('100')
  })

  it('should create a valid EGLD payload with scFunction', async () => {
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '100',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {
        scFunction: 'buy',
        arguments: ['01', '02'],
        relayer: relayer,
      },
    }

    const { payload } = await scheme.createPaymentPayload(1, req)
    const exactPayload = payload as ExactMultiversXPayload

    expect(exactPayload.data).toBe('buy@01@02')
    expect(exactPayload.receiver).toBe(bob)
    expect(exactPayload.value).toBe('100')
    expect(exactPayload.gasLimit).toBe(10_313_500) // Base(50k) + Data(9*1500) + Multi(200k) + Relayed(50k) + SC(10M)
  })

  it('should create a valid ESDT payload', async () => {
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '100',
      asset: 'TEST-123456',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {
        relayer: relayer,
      },
    }

    const { payload } = await scheme.createPaymentPayload(1, req)
    const exactPayload = payload as ExactMultiversXPayload

    expect(exactPayload.receiver).toBe(alice)
    expect(exactPayload.value).toBe('0')
    expect(exactPayload.data).toContain('MultiESDTNFTTransfer')
    expect(exactPayload.data).toContain(new Address(bob).hex())
    expect(exactPayload.data).toContain(Buffer.from('TEST-123456', 'utf8').toString('hex'))
    expect(exactPayload.gasLimit).toBe(10_475_500)
  })

  it('should handle EGLD-000000 alias as ESDT', async () => {
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '100',
      asset: 'EGLD-000000',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {
        relayer: relayer,
      },
    }

    const { payload } = await scheme.createPaymentPayload(1, req)
    const exactPayload = payload as ExactMultiversXPayload

    expect(exactPayload.value).toBe('0')
    expect(exactPayload.data).toContain('MultiESDTNFTTransfer')
    expect(exactPayload.data).toContain(Buffer.from('EGLD-000000', 'utf8').toString('hex'))
  })
})
