import { instance } from '@/api'

export interface DeliveryLog {
	postfix_message_id: string
	log_time: number
	status: string
	sender: string
	recipient: string
	mail_provider: string
	dsn: string
	relay: string
	delay: number
	description: string
	reason: string
}

export const getDeliveryLogs = (params: {
	status?: string
	keyword?: string
	start_time?: number
	end_time?: number
	page: number
	page_size: number
}) => {
	return instance.get('/delivery_log/list', { params })
}
