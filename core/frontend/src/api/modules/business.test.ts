import { describe, expect, it, vi } from 'vitest'
import { instance } from '@/api'
import { getBusinessProfile, saveBusinessProfile } from './business'
vi.mock('@/api', () => ({ instance: { get: vi.fn().mockResolvedValue({ profile: null }), put: vi.fn().mockResolvedValue({ revision: 1 }) } }))
describe('business profile', () => {
	it('requests the direct workspace resource', async () => {
		await getBusinessProfile()
		expect(instance.get).toHaveBeenCalledWith('/v1/business/profile', expect.objectContaining({ fetchOptions: { rawResponse: true } }))
	})
	it('preserves revision and mutation identity when saving', async () => {
		const profile = { name: 'Store', website: '', industry: '', description: '', brand_voice: 'Clear', locale: 'en', timezone: 'UTC', revision: 4 }
		await saveBusinessProfile(profile, 'retryable-request')
		expect(instance.put).toHaveBeenCalledWith('/v1/business/profile', profile, expect.objectContaining({ headers: { 'Idempotency-Key': 'retryable-request' } }))
	})
})
