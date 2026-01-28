import { describe, it, expect } from 'vitest'
import { ExactMultiversXServer } from '../src/exact/server/scheme'
import { PaymentRequirements } from '@x402/core/types'

describe('ExactMultiversXServer', () => {
    const server = new ExactMultiversXServer()

    describe('validatePaymentRequirements', () => {
        it('should throw if payTo is missing', () => {
            const req = { amount: '1000', asset: 'EGLD' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('PayTo is required')
        })

        it('should throw if payTo is invalid', () => {
            const req = { payTo: 'invalid', amount: '1000', asset: 'EGLD' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('invalid PayTo address')
        })

        it('should throw if amount is missing', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', asset: 'EGLD' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('amount is required')
        })

        it('should throw if amount is not numeric', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: 'abc', asset: 'EGLD' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('invalid amount')
        })

        it('should throw if asset is missing', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('asset is required')
        })

        it('should throw if ESDT asset is invalid', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000', asset: 'INVALID' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).toThrow('invalid asset TokenID')
        })

        it('should pass for valid EGLD requirements', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000', asset: 'EGLD' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).not.toThrow()
        })

        it('should pass for valid ESDT requirements', () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000', asset: 'TEST-123456' } as PaymentRequirements
            expect(() => server.validatePaymentRequirements(req)).not.toThrow()
        })
    })

    describe('enhancePaymentRequirements', () => {
        const supportedKind = {
            x402Version: 1,
            scheme: 'exact',
            network: 'multiversx:D' as any,
        }

        it('should enhance EGLD requirements with direct method and 50k gas', async () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000', asset: 'EGLD' } as PaymentRequirements
            const enhanced = await server.enhancePaymentRequirements(req, supportedKind, [])

            expect(enhanced.extra?.assetTransferMethod).toBe('direct')
            expect(enhanced.extra?.gasLimit).toBe(50_000)
        })

        it('should enhance ESDT requirements with esdt method and 60m gas', async () => {
            const req = { payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu', amount: '1000', asset: 'TEST-123456' } as PaymentRequirements
            const enhanced = await server.enhancePaymentRequirements(req, supportedKind, [])

            expect(enhanced.extra?.assetTransferMethod).toBe('esdt')
            expect(enhanced.extra?.gasLimit).toBe(60_000_000)
        })

        it('should preserve existing extra fields', async () => {
            const req = {
                payTo: 'erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu',
                amount: '1000',
                asset: 'EGLD',
                extra: { custom: 'field' }
            } as PaymentRequirements
            const enhanced = await server.enhancePaymentRequirements(req, supportedKind, [])

            expect(enhanced.extra?.custom).toBe('field')
            expect(enhanced.extra?.assetTransferMethod).toBe('direct')
        })
    })
})
