import { instance } from '@/api'

export interface SmsConfig {
	enabled: boolean
	provider: 'twilio' | 'generic'
	default_country_code: string
	twilio_account_sid: string
	twilio_auth_token: string
	twilio_from: string
	generic_method: 'GET' | 'POST'
	generic_url: string
	generic_body: string
	generic_content_type: string
	generic_api_key: string
	generic_sender: string
}

export interface SmsLog {
	id: number
	phone: string
	message: string
	provider: string
	status: 'sent' | 'failed'
	error: string
	group_id: number
	create_time: number
}

export const getSmsConfig = () => {
	return instance.get('/sms/config')
}

export const saveSmsConfig = (data: SmsConfig) => {
	return instance.post('/sms/config', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const sendTestSms = (data: { phone: string; message: string }) => {
	return instance.post('/sms/send_test', data, {
		fetchOptions: { loading: 'Sending...', successMessage: true },
	})
}

export const sendGroupSms = (data: { group_id: number; message: string }) => {
	return instance.post('/sms/send_group', data, {
		fetchOptions: { loading: 'Starting...', successMessage: true },
	})
}

export const getSmsLogs = (params: {
	status?: string | null
	keyword?: string
	page: number
	page_size: number
}) => {
	return instance.get('/sms/logs', { params })
}
