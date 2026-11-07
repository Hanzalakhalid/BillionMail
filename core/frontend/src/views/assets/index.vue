<template>
	<div class="p-6">
		<div class="flex justify-between items-center mb-5 gap-4">
			<div>
				<h1 class="text-xl font-semibold">Asset library</h1>
				<p class="text-sm text-gray-500">Upload product references and campaign media. Files become available after validation.</p>
			</div>
			<n-button :loading="loading" :disabled="loadingMore" @click="refresh()">Refresh</n-button>
		</div>
		<n-alert v-if="error" type="error" class="mb-4">{{ error }}</n-alert>
		<n-alert v-if="notice" type="info" class="mb-4">{{ notice }}</n-alert>
		<n-card v-if="canManage" title="Upload a file" class="mb-5">
			<div class="flex flex-wrap items-center gap-4">
				<input ref="fileInput" aria-label="Choose asset file" type="file" accept="image/png,image/jpeg,image/webp,video/mp4,application/pdf" :disabled="uploading" @change="selectFile" />
				<n-input v-model:value="title" aria-label="Asset title" placeholder="Asset title" :maxlength="200" :disabled="uploading" style="width: 260px" />
				<n-button type="primary" :loading="uploading" :disabled="!selected || !title.trim()" @click="upload">Upload</n-button>
				<n-button v-if="uploading" @click="uploadController?.abort()">Cancel</n-button>
			</div>
			<p class="text-sm text-gray-500 mt-3">PNG, JPEG, WebP, MP4 or PDF · Up to 20 MiB per file</p>
		</n-card>
		<n-spin :show="loading">
			<n-empty v-if="!items.length" description="No assets yet" />
			<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
				<n-card v-for="asset in items" :key="asset.id" :title="asset.title">
					<div class="flex items-center justify-between mb-3">
						<span class="text-sm text-gray-500">{{ asset.kind }}<span v-if="asset.byte_size"> · {{ formatBytes(asset.byte_size) }}</span></span>
						<n-tag :type="asset.status === 'ready' ? 'success' : ['rejected', 'failed'].includes(asset.status) ? 'error' : 'warning'">{{ statusLabels[asset.status] }}</n-tag>
					</div>
					<p v-if="asset.rejection_reason" class="text-sm text-red-600 mb-3">{{ asset.rejection_reason }}</p>
					<template v-if="urls[asset.version_id]">
						<img v-if="asset.kind === 'image'" :src="urls[asset.version_id]" :alt="asset.title" class="w-full max-h-64 object-contain mb-3" />
						<video v-else-if="asset.kind === 'video'" :src="urls[asset.version_id]" controls class="w-full mb-3" preload="metadata" />
						<a :href="urls[asset.version_id]" target="_blank" rel="noopener noreferrer" class="text-blue-600 mr-4">Open file</a>
					</template>
					<n-button v-if="asset.status === 'ready'" size="small" :loading="previewing === asset.version_id" @click="preview(asset)">Preview</n-button>
					<n-popconfirm v-if="canManage" @positive-click="remove(asset)">
						<template #trigger><n-button size="small" class="ml-3" type="error" ghost>Remove</n-button></template>
						Remove this asset from your library? Existing download links expire within five minutes.
					</n-popconfirm>
				</n-card>
			</div>
			<n-button v-if="nextCursor" class="mt-5" :loading="loadingMore" @click="loadMore">Load more</n-button>
		</n-spin>
	</div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { deleteAsset, getAssetURL, listAssets, uploadAsset, type Asset } from '@/api/modules/assets'
import { useCompanyStore } from '@/store'

const companyStore = useCompanyStore()
const canManage = computed(() => ['owner', 'admin', 'manager'].includes(companyStore.activeCompany?.role || ''))

const items = ref<Asset[]>([])
const urls = ref<Record<string, string>>({})
const loading = ref(false)
const loadingMore = ref(false)
const nextCursor = ref('')
let refreshing = false
const uploading = ref(false)
const error = ref('')
const notice = ref('')
const selected = ref<File | null>(null)
const title = ref('')
const previewing = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
let idempotencyKey = crypto.randomUUID()
let uploadController: AbortController | undefined
let poll: ReturnType<typeof setInterval> | undefined
const statusLabels: Record<Asset['status'], string> = { awaiting_upload: 'Awaiting upload', processing: 'Validating', ready: 'Ready', rejected: 'Rejected', failed: 'Failed' }
watch(title, () => { if (!uploading.value) idempotencyKey = crypto.randomUUID() })

function message(cause: unknown): string {
	const response = (cause as { response?: { data?: { msg?: unknown } } } | null)?.response
	if (response?.data?.msg) return String(response.data.msg)
	if (cause && typeof cause === 'object' && 'msg' in cause) return String(cause.msg)
	return cause instanceof Error ? cause.message : String(cause)
}
function formatBytes(size: number): string { return `${(size / 1024 / 1024).toFixed(2)} MiB` }
function selectFile(event: Event) {
	error.value = ''; notice.value = ''
	const file = (event.target as HTMLInputElement).files?.[0]
	selected.value = null
	if (!file) return
	if (!['image/png', 'image/jpeg', 'image/webp', 'video/mp4', 'application/pdf'].includes(file.type) || file.size < 1 || file.size > 20 * 1024 * 1024) {
		error.value = 'Choose a supported file up to 20 MiB.'
		return
	}
	selected.value = file; title.value = file.name; idempotencyKey = crypto.randomUUID()
}
async function refresh(quiet = false) {
	if (refreshing || loadingMore.value) return
	refreshing = true
	if (!quiet) loading.value = true
	if (!quiet) error.value = ''
	try {
		const page = await listAssets()
		if (quiet) {
			const updates = new Map(page.items.map(asset => [asset.id, asset]))
			const existing = new Set(items.value.map(asset => asset.id))
			items.value = [...page.items.filter(asset => !existing.has(asset.id)), ...items.value.map(asset => updates.get(asset.id) || asset)]
		} else { items.value = page.items; nextCursor.value = page.next_cursor }
	}
	catch (cause) { error.value = message(cause) }
	finally { refreshing = false; if (!quiet) loading.value = false }
}
async function loadMore() {
	if (!nextCursor.value || loadingMore.value || refreshing) return
	loadingMore.value = true
	try {
		const page = await listAssets(nextCursor.value)
		const existing = new Set(items.value.map(asset => asset.id))
		items.value.push(...page.items.filter(asset => !existing.has(asset.id)))
		nextCursor.value = page.next_cursor
	} catch (cause) { error.value = message(cause) }
	finally { loadingMore.value = false }
}
async function remove(asset: Asset) {
	try {
		await deleteAsset(asset.id)
		items.value = items.value.filter(item => item.id !== asset.id)
		delete urls.value[asset.version_id]
	} catch (cause) { error.value = message(cause) }
}
async function upload() {
	if (!selected.value || uploading.value || !title.value.trim()) return
	uploading.value = true; error.value = ''; notice.value = ''
	uploadController = new AbortController()
	try {
		await uploadAsset(selected.value, title.value.trim(), idempotencyKey, uploadController.signal)
		notice.value = 'Upload received. The file will appear as Ready after validation.'
		selected.value = null; title.value = ''
		if (fileInput.value) fileInput.value.value = ''
		await refresh()
	} catch (cause) { error.value = uploadController.signal.aborted ? 'Upload cancelled.' : message(cause) }
	finally { uploading.value = false; uploadController = undefined }
}
async function preview(asset: Asset) {
	previewing.value = asset.version_id
	try { urls.value[asset.version_id] = (await getAssetURL(asset.version_id)).url }
	catch (cause) { error.value = message(cause) }
	finally { previewing.value = '' }
}
onMounted(() => {
	refresh()
	poll = setInterval(() => { if (items.value.some(asset => asset.status === 'processing')) refresh(true) }, 5000)
})
onUnmounted(() => { if (poll) clearInterval(poll); uploadController?.abort() })
</script>
