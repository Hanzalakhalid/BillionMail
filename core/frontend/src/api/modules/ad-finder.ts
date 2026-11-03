import { instance } from '@/api'

export interface AdFinderConfig {
	apify_token: string
	default_country: string
	results_limit: number
}

export interface Advertiser {
	page_id: string
	page_name: string
	page_url: string
	ad_library_url: string
	website: string
	category: string
	ad_topic: string
	ad_text: string
	cta: string
	ad_count: number
	days_running: number
	platforms: string
	score: number
	reasons: string
	keyword: string
	country: string
	lead_email: string
	update_time: number
}

export const getAdFinderConfig = () => {
	return instance.get('/ad_intel/config')
}

export const saveAdFinderConfig = (data: AdFinderConfig) => {
	return instance.post('/ad_intel/config', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const searchAds = (data: { keyword: string; country: string; limit: number }) => {
	return instance.post('/ad_intel/search', data, {
		fetchOptions: { loading: 'Fetching ads from Facebook, this can take a minute...', successMessage: true },
	})
}

export const listAdvertisers = (params: { keyword?: string | null; search?: string; page: number; page_size: number }) => {
	return instance.get('/ad_intel/list', { params })
}

export const addAdvertiserLead = (data: { page_id: string; group_id: number; email?: string }) => {
	return instance.post('/ad_intel/add_lead', data, {
		fetchOptions: { loading: 'Finding email and adding...', successMessage: true },
	})
}
