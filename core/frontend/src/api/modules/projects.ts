import { instance } from '@/api'

export interface ProjectRequirements {
	audience: string
	channels: string[]
	languages: string[]
	success_metric: string
	constraints: string
}
export interface ProjectInput {
	id: string
	revision: number
	title: string
	objective: string
	requirements: ProjectRequirements
}
export interface MarketingProject extends ProjectInput {
	status: string
	missing: string[]
	created_at: string
	updated_at: string
}
export interface ProjectMessage { id: string; role: string; content: string; created_at: string }
export interface ProjectEvent { id: number; type: string; payload: MarketingProject | ProjectMessage; created_at: string }
const direct = { fetchOptions: { rawResponse: true } }
export const listProjects = (cursor = '') => instance.get<{ items: MarketingProject[]; next_cursor: string }, { items: MarketingProject[]; next_cursor: string }>('/v1/projects/list', { ...direct, params: { cursor } })
export const getProject = (id: string) => instance.get<MarketingProject, MarketingProject>('/v1/projects/get', { ...direct, params: { id } })
export const saveProject = (input: ProjectInput, key: string) => instance.post<MarketingProject, MarketingProject>('/v1/projects/save', input, { ...direct, headers: { 'Idempotency-Key': key } })
export const addProjectMessage = (id: string, content: string, key: string) => instance.post<ProjectMessage, ProjectMessage>('/v1/projects/messages', { id, content }, { ...direct, headers: { 'Idempotency-Key': key } })
export const projectEvents = (id: string, after = 0) => instance.get<{ items: ProjectEvent[]; next_after: number }, { items: ProjectEvent[]; next_after: number }>('/v1/projects/events', { ...direct, params: { id, after } })
