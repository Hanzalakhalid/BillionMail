import { instance } from '@/api'

export interface ProductInput {
	id?: string
	revision: number
	name: string
	sku: string
	description: string
	benefits: string[]
	url: string
	price: string
	currency: string
	asset_version_ids: string[]
}
export interface Product extends ProductInput { id: string; version_id: string; created_at: string; updated_at: string }
export interface ProductPage { items: Product[]; next_cursor: string }
export interface ProductVersion { id: string; version: number; product: Product; created_at: string }
const direct = { fetchOptions: { rawResponse: true } }
export const listProducts = (cursor = '') => instance.get<ProductPage, ProductPage>('/v1/products/list', { ...direct, params: { cursor } })
export const getProduct = (id: string) => instance.get<Product, Product>('/v1/products/get', { ...direct, params: { id } })
export const saveProduct = (input: ProductInput, key: string) => instance.post<Product, Product>('/v1/products/save', input, { ...direct, headers: { 'Idempotency-Key': key } })
export const productVersions = (id: string, before = 0) => instance.get<{ items: ProductVersion[]; next_before: number }, { items: ProductVersion[]; next_before: number }>('/v1/products/versions', { ...direct, params: { id, before } })
export const deleteProduct = (id: string) => instance.post('/v1/products/delete', { id }, { ...direct, headers: { 'Idempotency-Key': `delete:${id}` } })
