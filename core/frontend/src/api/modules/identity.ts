import { instance } from '@/api'

const publicRequest = { fetchOptions: { successMessage: true } }

export interface RegistrationRequest {
	username: string
	email: string
	password: string
	company_name: string
	timezone: string
	locale: string
}

export const register = (data: RegistrationRequest) =>
	instance.post('/v1/auth/register', data, publicRequest)

export const verifyEmail = (token: string) =>
	instance.post('/v1/auth/verify-email', { token }, publicRequest)

export const requestPasswordReset = (email: string) =>
	instance.post('/v1/auth/forgot-password', { email }, publicRequest)

export const resetPassword = (token: string, password: string) =>
	instance.post('/v1/auth/reset-password', { token, password }, publicRequest)
