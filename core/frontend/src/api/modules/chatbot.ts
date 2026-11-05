import { instance } from '@/api'

export interface BotOption {
	id: string
	label: string
	value: string
}
export interface BotNode {
	id: string
	type: 'trigger' | 'message' | 'menu' | 'end'
	position: { x: number; y: number }
	data: { text?: string; options?: BotOption[] }
}
export interface BotEdge {
	id: string
	source: string
	target: string
	source_handle?: string
}
export interface BotGraph {
	schema_version: 1
	nodes: BotNode[]
	edges: BotEdge[]
}
export interface ChatbotFlow {
	id: string
	company_id: string
	phone_number_id: string
	display_phone_number: string
	verified_name: string
	name: string
	status: 'draft' | 'active' | 'paused'
	draft_version: number
	published_version: number
	graph: BotGraph
	created_at: string
	updated_at: string
}
export interface ChatbotPayload {
	name: string
	phone_number_id: string
	graph: BotGraph
}

export const listChatbots = () => instance.get('/v1/chatbots')
export const getChatbot = (id: string) => instance.get(`/v1/chatbots/${id}`)
export const createChatbot = (data: ChatbotPayload) =>
	instance.post('/v1/chatbots', data, {
		fetchOptions: { loading: 'Saving bot...', successMessage: true },
	})
export const updateChatbot = (id: string, data: ChatbotPayload) =>
	instance.put(`/v1/chatbots/${id}`, data, {
		fetchOptions: { loading: 'Saving bot...', successMessage: true },
	})
export const publishChatbot = (id: string) =>
	instance.post(
		`/v1/chatbots/${id}/publish`,
		{},
		{ fetchOptions: { loading: 'Publishing bot...', successMessage: true } }
	)
export const deleteChatbot = (id: string) =>
	instance.delete(`/v1/chatbots/${id}`, {
		fetchOptions: { loading: 'Deleting bot...', successMessage: true },
	})
