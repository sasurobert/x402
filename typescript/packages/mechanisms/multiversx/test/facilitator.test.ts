import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ExactMultiversXFacilitator } from '../src/exact/facilitator/scheme'
import { PaymentRequirements, PaymentPayload } from '@x402/core/types'
import { ExactMultiversXPayload } from '../src/types'
import { Address } from '@multiversx/sdk-core'

const alice = 'erd1qy9evls968sh2lg89f4yfq9jfsy68xsywv3sh42rg7496sk99cqsyat6wa'
const bob = 'erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx'

const mockResource: PaymentPayload['resource'] = {
  url: 'https://example.com/res',
  description: 'Test Resource',
  mimeType: 'text/plain',
}

describe('ExactMultiversXFacilitator', () => {
  let facilitator: ExactMultiversXFacilitator

  beforeEach(() => {
    facilitator = new ExactMultiversXFacilitator('https://mock-api.com')
    // Mock global fetch
    global.fetch = vi.fn()
  })

  it('should verify a valid EGLD payload', async () => {
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

    // Mock successful simulation
    ;(global.fetch as any).mockResolvedValue({
      ok: true,
      headers: { get: () => 'application/json' },
      json: async () => ({
        data: { result: { status: 'success', hash: 'tx-hash' } },
        code: 'successful',
      }),
    })

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid).toBe(true)
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
      validBefore: Math.floor(Date.now() / 1000) - 100, // Expired
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
      value: '500', // too low
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

    // Mock send response
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: { txHash: 'tx-123' } }),
    })

    // Mock wait status response (success immediately)
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: { status: 'success' } }),
    })

    const result = await facilitator.settle(fullPayload, req)
    expect(result.success).toBe(true)
    expect(result.transaction).toBe('tx-123')
  })

  it('should verify a valid ESDT payload', async () => {
    const asset = 'TEST-123456'
    const amount = '1000'
    const amountHex = BigInt(amount).toString(16).padStart(2, '0')
    const tokenHex = Buffer.from(asset, 'utf8').toString('hex')
    const destHex = new Address(bob).hex()

    // MultiESDTNFTTransfer @ DestHex @ 01 @ TokenHex @ 00 @ AmountHex
    const data = `MultiESDTNFTTransfer@${destHex}@01@${tokenHex}@00@${amountHex}`

    const payload: ExactMultiversXPayload = {
      nonce: 10,
      value: '0',
      receiver: alice, // Self
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

    // Mock successful simulation
    ;(global.fetch as any).mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { result: { status: 'success' } },
      }),
    })

    const result = await facilitator.verify(fullPayload, req)
    expect(result.isValid).toBe(true)
  })
})
