import { beforeEach, describe, expect, it, vi } from 'vitest'
import { instance } from '@/api'
import { uploadAsset } from './assets'

vi.mock('@/api', () => ({ instance: { post: vi.fn(), get: vi.fn() } }))

describe('asset upload handoff', () => {
	beforeEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals() })
	const file = new File(['product image'], 'product.png', { type: 'image/png' })
	const form = { asset_id: 'asset', version_id: 'version', status: 'awaiting_upload', form: { url: 'https://storage.test/upload', fields: { key: 'quarantine/key', policy: 'signed-policy' } } }
	it('queues validation only after storage accepts the file', async () => {
		vi.mocked(instance.post).mockResolvedValueOnce(form).mockResolvedValueOnce({ job_id: 'job', resource_id: 'asset', status: 'queued', resource_status: 'processing' })
		const fetchMock = vi.fn().mockResolvedValue({ ok: true })
		vi.stubGlobal('fetch', fetchMock)
		await expect(uploadAsset(file, 'Product', 'request-key')).resolves.toEqual({ job_id: 'job', resource_id: 'asset', status: 'queued', resource_status: 'processing' })
		expect(fetchMock).toHaveBeenCalledOnce()
		const request = fetchMock.mock.calls[0][1]
		expect(request.body.get('policy')).toBe('signed-policy')
		expect(request.body.get('file')).toBeInstanceOf(File)
		expect(request.headers).toBeUndefined()
		expect(instance.post).toHaveBeenLastCalledWith('/v1/assets/uploads/complete', { version_id: 'version' }, expect.objectContaining({ headers: { 'Idempotency-Key': 'request-key:complete' } }))
	})
	it('does not enqueue validation after a failed storage upload', async () => {
		vi.mocked(instance.post).mockResolvedValueOnce(form)
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 403 }))
		await expect(uploadAsset(file, 'Product', 'request-key')).rejects.toThrow('Upload failed')
		expect(instance.post).toHaveBeenCalledTimes(1)
	})
	it('resumes an already queued upload without resending bytes', async () => {
		vi.mocked(instance.post).mockResolvedValueOnce({ ...form, status: 'processing', form: undefined }).mockResolvedValueOnce({ job_id: 'job', resource_id: 'asset', status: 'processing' })
		const fetchMock = vi.fn(); vi.stubGlobal('fetch', fetchMock)
		await uploadAsset(file, 'Product', 'same-key')
		expect(fetchMock).not.toHaveBeenCalled()
	})
})
