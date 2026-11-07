import { describe, expect, it, vi } from 'vitest'
import { instance } from '@/api'
import { listProducts, saveProduct, productVersions, deleteProduct } from './products'
vi.mock('@/api', () => ({ instance: { get: vi.fn(), post: vi.fn() } }))
describe('product catalog API', () => {
	it('keeps cursor and direct resource format', async () => {
		await listProducts('page-cursor')
		expect(instance.get).toHaveBeenCalledWith('/v1/products/list', expect.objectContaining({ params: { cursor: 'page-cursor' }, fetchOptions: { rawResponse: true } }))
	})
	it('sends exact product revision and request identity', async () => {
		const input = { id: 'product-id', revision: 3, name: 'Product', sku: '', description: '', benefits: [], url: '', price: '12.99', currency: 'USD', asset_version_ids: ['version-id'] }
		await saveProduct(input, 'save-key')
		expect(instance.post).toHaveBeenCalledWith('/v1/products/save', input, expect.objectContaining({ headers: { 'Idempotency-Key': 'save-key' } }))
	})
	it('pages older immutable versions and makes deletion retryable', async () => {
		await productVersions('product-id', 12)
		expect(instance.get).toHaveBeenCalledWith('/v1/products/versions', expect.objectContaining({ params: { id: 'product-id', before: 12 } }))
		await deleteProduct('product-id')
		expect(instance.post).toHaveBeenCalledWith('/v1/products/delete', { id: 'product-id' }, expect.objectContaining({ headers: { 'Idempotency-Key': 'delete:product-id' } }))
	})
})
