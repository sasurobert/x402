import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ExactMultiversXFacilitator } from '../src/exact/facilitator/scheme'
import { PaymentRequirements, PaymentPayload } from '@x402/core/types'
import { ExactMultiversXPayload } from '../src/types'
import { Address } from '@multiversx/sdk-core'

const mockProvider = {
  simulateTransaction: vi.fn(),
  sendTransaction: vi.fn(),
  getTransactionStatus: vi.fn(),
  awaitTransactionCompleted: vi.fn(),
}

vi.mock('@multiversx/sdk-network-providers', () => ({
  ApiNetworkProvider: vi.fn().mockImplementation(() => mockProvider),
}))

const alice = 'erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th'
const bob = 'erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx'

const mockResource: PaymentPayload['resource'] = {
  url: 'https://example.com/res',
  description: 'Test Resource',
  mimeType: 'text/plain',
}

describe('ExactMultiversXFacilitator', () => {
  let facilitator: ExactMultiversXFacilitator

  beforeEach(() => {
    vi.clearAllMocks()
    facilitator = new ExactMultiversXFacilitator('https://mock-api.com')
  })

  it('should verify a valid EGLD payload', async () => {
    vi.spyOn(facilitator as any, 'verifySignature').mockResolvedValue({ isValid: true })
    mockProvider.simulateTransaction.mockResolvedValue({
      status: 'success',
    })

    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '1000',
      receiver: bob,
      sender: alice,
      gasPrice: 1000000000,
      gasLimit: 50000,
      chainID: 'D',
      version: 2,
      signature: 'sig',
      validAfter: Math.floor(Date.now() / 1000) - 100,
      validBefore: Math.floor(Date.now() / 1000) + 100,
    }

    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {},
    }

    const fullPayload: PaymentPayload = {
      x402Version: 1,
      resource: mockResource,
      accepted: req,
      payload: payload as any,
    }

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid, result.invalidReason).toBe(true)
    expect(mockProvider.simulateTransaction).toHaveBeenCalled()
  })

  it('should fail if expired', async () => {
    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '1000',
      receiver: bob,
      sender: alice,
      gasPrice: 1000000000,
      gasLimit: 50000,
      chainID: 'D',
      version: 2,
      validBefore: Math.floor(Date.now() / 1000) - 100,
    }
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {},
    }

    const fullPayload: PaymentPayload = {
      x402Version: 1,
      resource: mockResource,
      accepted: req,
      payload: payload as any,
    }

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid).toBe(false)
    expect(result.invalidReason).toContain('expired')
  })

  it('should fail if verification logic mismatches', async () => {
    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '500',
      receiver: bob,
      sender: alice,
      gasPrice: 1000000000,
      gasLimit: 50000,
      chainID: 'D',
      version: 2,
    }
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {},
    }

    const fullPayload: PaymentPayload = {
      x402Version: 1,
      resource: mockResource,
      accepted: req,
      payload: payload as any,
    }

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid).toBe(false)
    expect(result.invalidReason).toContain('Amount too low')
  })

  it('should wait for transaction success on settle', async () => {
    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '1000',
      receiver: bob,
      sender: alice,
      gasPrice: 1000000000,
      gasLimit: 50000,
      chainID: 'D',
      version: 2,
      signature: 'sig',
    }
    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount: '1000',
      asset: 'EGLD',
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {},
    }

    const fullPayload: PaymentPayload = {
      x402Version: 1,
      resource: mockResource,
      accepted: req,
      payload: payload as any,
    }

    mockProvider.sendTransaction.mockResolvedValue('tx-123')
    mockProvider.awaitTransactionCompleted.mockResolvedValue({})

    const result = await facilitator.settle(fullPayload, req)
    expect(result.success, result.errorReason).toBe(true)
    expect(result.transaction).toBe('tx-123')
    expect(mockProvider.sendTransaction).toHaveBeenCalled()
    expect(mockProvider.awaitTransactionCompleted).toHaveBeenCalledWith('tx-123')
  })

  it('should verify a valid ESDT payload', async () => {
    vi.spyOn(facilitator as any, 'verifySignature').mockResolvedValue({ isValid: true })
    mockProvider.simulateTransaction.mockResolvedValue({
      status: 'success',
    })

    const asset = 'TEST-123456'
    const amount = '1000'
    const amountHex = BigInt(amount).toString(16).padStart(2, '0')
    const tokenHex = Buffer.from(asset, 'utf8').toString('hex')
    const destHex = new Address(bob).hex()

    const data = `MultiESDTNFTTransfer@${destHex}@01@${tokenHex}@00@${amountHex}`

    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '0',
      receiver: alice,
      sender: alice,
      gasPrice: 1000000000,
      gasLimit: 60000000,
      data,
      chainID: 'D',
      version: 2,
      signature: 'sig',
    }

    const req: PaymentRequirements = {
      scheme: 'exact',
      payTo: bob,
      amount,
      asset,
      network: 'multiversx:D',
      maxTimeoutSeconds: 0,
      extra: {},
    }

    const fullPayload: PaymentPayload = {
      x402Version: 1,
      resource: mockResource,
      accepted: req,
      payload: payload as any,
    }

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid, result.invalidReason).toBe(true)
  })
})
