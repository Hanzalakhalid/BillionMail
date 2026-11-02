import { instance } from '@/api'

export interface WhatsappConfig {
	enabled: boolean
	phone_number_id: string
	business_account_id: string
	access_token: string
	api_version: string
	default_country_code: string
}

export interface WhatsappTemplate {
	name: string
	language: string
	status: string
	category: string
	body: string
}

export interface WhatsappLog {
	id: number
	phone: string
	template: string
	params: string
	message_id: string
	status: 'sent' | 'failed'
	error: string
	group_id: number
	create_time: number
}

interface TemplateFields {
	template: string
	language: string
	params: string[]
}

export const getWhatsappConfig = () => {
	return instance.get('/whatsapp/config')
}

export const saveWhatsappConfig = (data: WhatsappConfig) => {
	return instance.post('/whatsapp/config', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const getWhatsappTemplates = () => {
	return instance.get('/whatsapp/templates')
}

export const sendWhatsappTest = (data: TemplateFields & { phone: string }) => {
	return instance.post('/whatsapp/send_test', data, {
		fetchOptions: { loading: 'Sending...', successMessage: true },
	})
}

export const sendWhatsappGroup = (data: TemplateFields & { group_id: number }) => {
	return instance.post('/whatsapp/send_group', data, {
		fetchOptions: { loading: 'Starting...', successMessage: true },
	})
}

export const getWhatsappLogs = (params: {
	status?: string | null
	keyword?: string
	page: number
	page_size: number
}) => {
	return instance.get('/whatsapp/logs', { params })
}
