<template>
	<div class="p-6">
		<div class="flex justify-between items-center mb-5 gap-4">
			<div><h1 class="text-xl font-semibold">Products</h1><p class="text-sm text-gray-500">Save product facts, benefits, and reference images for your marketing projects.</p></div>
			<div class="flex gap-3"><n-button :loading="loading" @click="refresh">Refresh</n-button><n-button v-if="canManage" type="primary" @click="openEditor()">Add product</n-button></div>
		</div>
		<n-alert v-if="error" type="error" class="mb-4">{{ error }}</n-alert>
		<n-spin :show="loading">
			<n-empty v-if="!items.length" description="No products yet" />
			<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
				<n-card v-for="product in items" :key="product.id" :title="product.name">
					<div class="text-sm text-gray-500 mb-3">{{ product.sku || 'No SKU' }} · Version {{ product.revision }}<span v-if="product.price"> · {{ product.price }} {{ product.currency }}</span></div>
					<p class="mb-3 whitespace-pre-wrap line-clamp-4">{{ product.description }}</p>
					<p v-if="product.asset_version_ids.length" class="text-sm text-gray-500 mb-3">{{ product.asset_version_ids.length }} reference image(s)</p>
					<div class="flex gap-3 flex-wrap">
						<n-button v-if="canManage" size="small" @click="openEditor(product)">Edit</n-button>
						<n-button size="small" @click="openHistory(product)">Version history</n-button>
						<n-popconfirm v-if="canManage" @positive-click="remove(product)"><template #trigger><n-button size="small" type="error" ghost>Remove</n-button></template>Remove this product from the catalog?</n-popconfirm>
					</div>
				</n-card>
			</div>
			<n-button v-if="nextCursor" class="mt-5" :loading="loadingMore" @click="loadMore">Load more</n-button>
		</n-spin>
		<n-modal v-model:show="editing" preset="card" :title="form.id ? 'Edit product' : 'Add product'" style="width: min(720px, 95vw)" :mask-closable="!saving" :closable="!saving">
			<n-alert v-if="editorError" type="error" class="mb-4">{{ editorError }}</n-alert>
			<n-form label-placement="top" :disabled="saving" @submit.prevent="save">
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<n-form-item label="Product name" required><n-input v-model:value="form.name" :maxlength="200" /></n-form-item>
					<n-form-item label="SKU"><n-input v-model:value="form.sku" :maxlength="160" /></n-form-item>
				</div>
				<n-form-item label="Description"><n-input v-model:value="form.description" type="textarea" :maxlength="16000" :autosize="{ minRows: 3, maxRows: 8 }" /></n-form-item>
				<n-form-item label="Benefits (one per line)"><n-input v-model:value="benefitsText" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="What does the product help your customer do?" /></n-form-item>
				<n-form-item label="Product URL"><n-input v-model:value="form.url" :maxlength="2048" placeholder="https://example.com/product" /></n-form-item>
				<p class="text-sm text-gray-500 mb-4">The URL is saved as a reference. Enter the product facts and benefits above.</p>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<n-form-item label="Price (optional)"><n-input v-model:value="form.price" placeholder="24.95" /></n-form-item>
					<n-form-item label="Currency"><n-input v-model:value="form.currency" :maxlength="3" placeholder="USD" /></n-form-item>
				</div>
				<n-form-item label="Validated reference images"><n-select v-model:value="form.asset_version_ids" multiple filterable :max-tag-count="3" :options="referenceOptions" :loading="loadingReferences" /></n-form-item>
				<n-button v-if="referenceCursor" class="mb-4" size="small" :loading="loadingReferences" @click="loadReferences(true)">Load more reference images</n-button>
				<p class="text-sm text-gray-500 mb-4">Choose images from the Asset Library after they become Ready. Up to 20 images per product.</p>
				<n-button type="primary" :loading="saving" :disabled="!form.name.trim()" @click="save">Save product version</n-button>
			</n-form>
		</n-modal>
		<n-drawer v-model:show="showHistory" :width="560" :auto-focus="false"><n-drawer-content :title="historyTitle" closable>
			<n-alert v-if="historyError" type="error" class="mb-4">{{ historyError }}</n-alert>
			<n-spin :show="loadingHistory">
				<n-card v-for="version in history" :key="version.id" :title="`Version ${version.version}`" class="mb-4">
					<p class="text-sm text-gray-500 mb-3">{{ new Date(version.created_at).toLocaleString() }}</p>
					<p class="font-semibold">{{ version.product.name }}</p>
					<p class="whitespace-pre-wrap mb-3">{{ version.product.description }}</p>
					<ul v-if="version.product.benefits.length" class="list-disc pl-5 mb-3"><li v-for="(benefit, index) in version.product.benefits" :key="index">{{ benefit }}</li></ul>
					<p v-if="version.product.price">{{ version.product.price }} {{ version.product.currency }}</p>
					<p class="text-sm text-gray-500">{{ version.product.asset_version_ids.length }} reference image(s)</p>
				</n-card>
				<n-empty v-if="!history.length" description="No versions available" />
				<n-button v-if="historyBefore" :loading="loadingHistory" @click="loadHistory(true)">Older versions</n-button>
			</n-spin>
		</n-drawer-content></n-drawer>
	</div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useCompanyStore } from '@/store'
import { listAssets, type Asset } from '@/api/modules/assets'
import { deleteProduct, listProducts, productVersions, saveProduct, type Product, type ProductInput, type ProductVersion } from '@/api/modules/products'

const companyStore = useCompanyStore()
const canManage = computed(() => ['owner', 'admin', 'manager'].includes(companyStore.activeCompany?.role || ''))
const items = ref<Product[]>([])
const nextCursor = ref('')
const error = ref('')
const loading = ref(false)
const loadingMore = ref(false)
const editing = ref(false)
const saving = ref(false)
const editorError = ref('')
const empty = (): ProductInput => ({ revision: 0, name: '', sku: '', description: '', benefits: [], url: '', price: '', currency: '', asset_version_ids: [] })
const form = reactive<ProductInput>(empty())
const benefitsText = ref('')
let saveKey = crypto.randomUUID()
watch(() => JSON.stringify({ ...form, benefitsText: benefitsText.value }), () => { if (!saving.value) saveKey = crypto.randomUUID() })
const references = ref<Asset[]>([])
const referenceCursor = ref('')
const loadingReferences = ref(false)
const referenceOptions = computed(() => {
	const options = references.value.map(asset => ({ label: asset.title, value: asset.version_id }))
	const loaded = new Set(options.map(option => option.value))
	form.asset_version_ids.forEach((id, index) => { if (!loaded.has(id)) options.push({ label: `Saved reference image ${index + 1}`, value: id }) })
	return options
})
const showHistory = ref(false)
const historyTitle = ref('Product history')
const history = ref<ProductVersion[]>([])
const historyError = ref('')
const loadingHistory = ref(false)
const historyBefore = ref(0)
let historyID = ''

function message(cause: unknown): string {
	const response = (cause as { response?: { data?: { msg?: unknown }; status?: number } } | null)?.response
	if (response?.status === 409) return 'This product changed or its SKU is in use. Reopen the product to review the latest version.'
	if (response?.data?.msg) return String(response.data.msg)
	if (cause && typeof cause === 'object' && 'msg' in cause) return String(cause.msg)
	return cause instanceof Error ? cause.message : 'The product request failed.'
}
async function refresh() {
	if (loading.value || loadingMore.value) return
	loading.value = true; error.value = ''
	try { const page = await listProducts(); items.value = page.items; nextCursor.value = page.next_cursor }
	catch (cause) { error.value = message(cause) }
	finally { loading.value = false }
}
async function loadMore() {
	if (!nextCursor.value || loadingMore.value || loading.value) return
	loadingMore.value = true
	try { const page = await listProducts(nextCursor.value); const ids = new Set(items.value.map(item => item.id)); items.value.push(...page.items.filter(item => !ids.has(item.id))); nextCursor.value = page.next_cursor }
	catch (cause) { error.value = message(cause) }
	finally { loadingMore.value = false }
}
async function loadReferences(more = false) {
	if (loadingReferences.value) return
	loadingReferences.value = true
	try { const page = await listAssets(more ? referenceCursor.value : ''); const images = page.items.filter(asset => asset.status === 'ready' && asset.kind === 'image'); references.value = more ? [...references.value, ...images] : images; referenceCursor.value = page.next_cursor }
	catch (cause) { editorError.value = message(cause) }
	finally { loadingReferences.value = false }
}
function openEditor(product?: Product) {
	delete form.id
	Object.assign(form, product ? { id: product.id, revision: product.revision, name: product.name, sku: product.sku, description: product.description, benefits: [...product.benefits], url: product.url, price: product.price, currency: product.currency, asset_version_ids: [...product.asset_version_ids] } : empty())
	benefitsText.value = form.benefits.join('\n'); saveKey = crypto.randomUUID(); editorError.value = ''; editing.value = true
	loadReferences()
}
async function save() {
	if (saving.value || !canManage.value || !form.name.trim()) return
	if (form.asset_version_ids.length > 20) { editorError.value = 'Choose up to 20 reference images.'; return }
	saving.value = true; editorError.value = ''
	try {
		await saveProduct({ ...form, name: form.name.trim(), sku: form.sku.trim(), benefits: benefitsText.value.split('\n').map(value => value.trim()).filter(Boolean), price: form.price.trim(), currency: form.currency.trim().toUpperCase(), url: form.url.trim() }, saveKey)
		editing.value = false; await refresh()
	} catch (cause) { editorError.value = message(cause) }
	finally { saving.value = false }
}
async function remove(product: Product) {
	try { await deleteProduct(product.id); items.value = items.value.filter(item => item.id !== product.id) }
	catch (cause) { error.value = message(cause) }
}
function openHistory(product: Product) {
	historyID = product.id; historyTitle.value = `${product.name} · History`; history.value = []; historyBefore.value = 0; historyError.value = ''; showHistory.value = true
	loadHistory()
}
async function loadHistory(more = false) {
	if (loadingHistory.value) return
	const requestedID = historyID
	loadingHistory.value = true
	try { const page = await productVersions(requestedID, more ? historyBefore.value : 0); if (historyID !== requestedID) return; history.value = more ? [...history.value, ...page.items] : page.items; historyBefore.value = page.next_before }
	catch (cause) { historyError.value = message(cause) }
	finally { loadingHistory.value = false; if (historyID !== requestedID && showHistory.value) loadHistory() }
}
onMounted(refresh)
</script>
