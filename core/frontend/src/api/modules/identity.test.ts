import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api', () => ({ instance: { post } }))

import { register, requestPasswordReset, resetPassword, verifyEmail } from './identity'

describe('identity API', () => {
	beforeEach(() => post.mockClear())

	it('uses public identity endpoints', () => {
		const registration = {
			username: 'owner', email: 'owner@example.com', password: 'StrongPassword1',
			company_name: 'Acme', timezone: 'UTC', locale: 'en',
		}
		register(registration)
		verifyEmail('verification-token')
		requestPasswordReset('owner@example.com')
		resetPassword('reset-token', 'NewStrongPassword1')

		expect(post).toHaveBeenNthCalledWith(1, '/v1/auth/register', registration, expect.any(Object))
		expect(post).toHaveBeenNthCalledWith(2, '/v1/auth/verify-email', { token: 'verification-token' }, expect.any(Object))
		expect(post).toHaveBeenNthCalledWith(3, '/v1/auth/forgot-password', { email: 'owner@example.com' }, expect.any(Object))
		expect(post).toHaveBeenNthCalledWith(4, '/v1/auth/reset-password', { token: 'reset-token', password: 'NewStrongPassword1' }, expect.any(Object))
	})
})
