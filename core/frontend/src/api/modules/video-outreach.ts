import { instance } from '@/api'

export interface VideoStatus {
	contact_email: string
	group_id: number
	name: string
	lead_reasons: string
	lead_tier: string
	lead_score: string
	video_url: string
	thumbnail_url: string
	landing_page_url: string
	status: string
}

export const generateVideo = (data: { contact_email: string; group_id: number }) => {
	return instance.post('/video_outreach/generate', data)
}

export const getVideoStatus = (params: { contact_email: string; group_id: number }) => {
	return instance.get('/video_outreach/status', { params })
}

export const listVideoOutreach = (params: {
	group_id?: number
	tier?: string
	page: number
	page_size: number
}) => {
	return instance.get('/video_outreach/list', { params })
}

export const importLeads = (data: { group_id: number; content: string }) => {
	return instance.post('/video_outreach/import_leads', data, {
		fetchOptions: { loading: 'Importing and scoring...', successMessage: true },
	})
}

export const setVideo = (data: {
	contact_email: string
	group_id: number
	video_url: string
	thumbnail_url?: string
	cta_url?: string
}) => {
	return instance.post('/video_outreach/set_video', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}
