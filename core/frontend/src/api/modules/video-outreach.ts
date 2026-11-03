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
	views: number
	plays: number
	clicks: number
	last_activity: number
}

export interface VideoOutreachStats {
	ready_videos: number
	views: number
	plays: number
	clicks: number
	leads_viewed: number
	leads_clicked: number
}

export const getVideoOutreachStats = (params: { group_id?: number } = {}) => {
	return instance.get('/video_outreach/stats', { params })
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
