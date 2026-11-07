import { instance } from '@/api'

export interface BusinessInput {
	name: string
	website: string
	industry: string
	description: string
	brand_voice: string
	locale: string
	timezone: string
	revision: number
}
export interface BusinessProfile extends BusinessInput { id: string; created_at: string; updated_at: string }
const directResource = { fetchOptions: { rawResponse: true } }
export const getBusinessProfile = () => instance.get<{ profile: BusinessProfile | null }, { profile: BusinessProfile | null }>('/v1/business/profile', directResource)
export const saveBusinessProfile = (input: BusinessInput, key: string) => instance.put<BusinessProfile, BusinessProfile>('/v1/business/profile', input, { ...directResource, headers: { 'Idempotency-Key': key } })
