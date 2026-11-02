import { instance } from '@/api'

export interface PushConfig {
	enabled: boolean
	app_id: string
	rest_api_key: string
	default_url: string
}

export interface PushLog {
	id: number
	title: string
	message: string
	url: string
	audience: string
	targeted: number
	notification_id: string
	status: 'sent' | 'failed'
	error: string
	create_time: number
}

export interface PushSendParams {
	audience: 'all' | 'group'
	group_id?: number
	title: string
	message: string
	url: string
	image: string
}

export const getPushConfig = () => {
	return instance.get('/push/config')
}

export const savePushConfig = (data: PushConfig) => {
	return instance.post('/push/config', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const sendPush = (data: PushSendParams) => {
	return instance.post('/push/send', data, {
		fetchOptions: { loading: 'Sending...', successMessage: true },
	})
}

export const getPushLogs = (params: { status?: string | null; page: number; page_size: number }) => {
	return instance.get('/push/logs', { params })
}
