import { instance } from '@/api'

export interface Asset {
	id: string
	title: string
	kind: 'image' | 'video' | 'document'
	version_id: string
	status: 'awaiting_upload' | 'processing' | 'ready' | 'rejected' | 'failed'
	content_type: string
	byte_size: number
	width: number
	height: number
	rejection_reason: string
	created_at: string
}

interface UploadRequest {
	asset_id: string
	version_id: string
	status: Asset['status']
	expires_at: string
	form?: { url: string; fields: Record<string, string> }
}

export interface AssetJob { job_id: string; resource_id: string; status: 'queued' | 'leased' | 'running' | 'retrying' | 'succeeded' | 'failed' | 'dead-lettered' | 'cancelled'; resource_status: Asset['status'] }
const directResource = { fetchOptions: { rawResponse: true } }

export interface AssetPage { items: Asset[]; next_cursor: string }
export const listAssets = (cursor = '') => instance.get<AssetPage, AssetPage>('/v1/assets/list', { ...directResource, params: { cursor } })
export const deleteAsset = (id: string) => instance.post<void, void>('/v1/assets/delete', { id }, { ...directResource, headers: { 'Idempotency-Key': `delete:${id}` } })
export const getAssetURL = (versionID: string) =>
	instance.get<{ url: string }, { url: string }>('/v1/assets/download', { ...directResource, params: { version_id: versionID } })

export async function uploadAsset(file: File, title: string, key: string, signal?: AbortSignal): Promise<AssetJob> {
	const upload = await instance.post<UploadRequest, UploadRequest>('/v1/assets/uploads', {
		title,
		content_type: file.type,
		byte_size: file.size,
	}, { ...directResource, headers: { 'Idempotency-Key': key }, signal })
	if (upload.status === 'rejected' || upload.status === 'failed') throw new Error('This upload cannot be retried. Please choose the file again.')
	if (upload.status === 'awaiting_upload') {
		if (!upload.form) throw new Error('The upload form is unavailable. Please try again.')
		const body = new FormData()
		for (const [name, value] of Object.entries(upload.form.fields)) body.append(name, value)
		body.append('file', file)
		const response = await fetch(upload.form.url, { method: 'POST', body, signal })
		if (!response.ok) throw new Error('Upload failed. Please try again.')
	}
	return instance.post<AssetJob, AssetJob>('/v1/assets/uploads/complete', {
		version_id: upload.version_id,
	}, { ...directResource, headers: { 'Idempotency-Key': `${key}:complete` }, signal })
}
