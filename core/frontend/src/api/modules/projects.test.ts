import { describe, expect, it, vi } from 'vitest'
import { instance } from '@/api'
import { listProjects, saveProject, addProjectMessage, projectEvents } from './projects'
vi.mock('@/api', () => ({ instance: { get: vi.fn(), post: vi.fn() } }))

describe('persistent marketing projects', () => {
	it('uses tenant API pagination and direct resources', async () => {
		await listProjects('next')
		expect(instance.get).toHaveBeenCalledWith('/v1/projects/list', expect.objectContaining({ params: { cursor: 'next' }, fetchOptions: { rawResponse: true } }))
	})
	it('preserves discovery revisions and retry identity', async () => {
		const input = { id: '', revision: 0, title: 'Launch', objective: 'Launch a product', requirements: { audience: '', channels: ['email'], languages: ['en'], success_metric: '', constraints: '' } }
		await saveProject(input, 'save-key')
		expect(instance.post).toHaveBeenCalledWith('/v1/projects/save', input, expect.objectContaining({ headers: { 'Idempotency-Key': 'save-key' } }))
		await addProjectMessage('project', 'Warm tone', 'message-key')
		expect(instance.post).toHaveBeenCalledWith('/v1/projects/messages', { id: 'project', content: 'Warm tone' }, expect.objectContaining({ headers: { 'Idempotency-Key': 'message-key' } }))
	})
	it('recovers history after its durable cursor', async () => {
		await projectEvents('project', 42)
		expect(instance.get).toHaveBeenCalledWith('/v1/projects/events', expect.objectContaining({ params: { id: 'project', after: 42 } }))
	})
})
