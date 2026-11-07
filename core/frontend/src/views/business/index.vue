<template>
	<div class="p-6 max-w-4xl">
		<div class="flex justify-between items-center mb-5 gap-4">
			<div><h1 class="text-xl font-semibold">Business profile</h1><p class="text-sm text-gray-500">Give your marketing projects a consistent business context and brand voice.</p></div>
			<n-button :loading="loading" :disabled="saving" @click="load">Reload saved profile</n-button>
		</div>
		<n-alert v-if="error" type="error" class="mb-4">{{ error }}</n-alert>
		<n-alert v-if="saved" type="success" class="mb-4">Business profile saved.</n-alert>
		<n-spin :show="loading">
			<n-card>
				<n-form label-placement="top" :disabled="!canManage || saving" @submit.prevent="save">
					<n-form-item label="Business name" required><n-input v-model:value="form.name" :maxlength="160" /></n-form-item>
					<n-form-item label="Website"><n-input v-model:value="form.website" placeholder="https://example.com" :maxlength="2048" /></n-form-item>
					<n-form-item label="Industry"><n-input v-model:value="form.industry" :maxlength="160" /></n-form-item>
					<n-form-item label="What your business offers"><n-input v-model:value="form.description" type="textarea" :maxlength="16000" :autosize="{ minRows: 4, maxRows: 10 }" placeholder="Describe your products, customers, and what makes your business useful." /></n-form-item>
					<n-form-item label="Brand voice"><n-input v-model:value="form.brand_voice" type="textarea" :maxlength="8000" :autosize="{ minRows: 3, maxRows: 8 }" placeholder="Describe your tone, preferred wording, and language to avoid." /></n-form-item>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<n-form-item label="Default language" required><n-input v-model:value="form.locale" placeholder="en or en-US" :maxlength="32" /></n-form-item>
						<n-form-item label="Business timezone" required><n-input v-model:value="form.timezone" placeholder="Asia/Karachi" :maxlength="64" /></n-form-item>
					</div>
					<n-button v-if="canManage" type="primary" :loading="saving" :disabled="loading || !form.name.trim() || !form.locale || !form.timezone" @click="save">Save profile</n-button>
				</n-form>
			</n-card>
		</n-spin>
	</div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { getBusinessProfile, saveBusinessProfile, type BusinessInput } from '@/api/modules/business'
import { useCompanyStore } from '@/store'

const companyStore = useCompanyStore()
const canManage = computed(() => ['owner', 'admin', 'manager'].includes(companyStore.activeCompany?.role || ''))
const form = reactive<BusinessInput>({ name: '', website: '', industry: '', description: '', brand_voice: '', locale: 'en', timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', revision: 0 })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const saved = ref(false)
let requestKey = crypto.randomUUID()
watch(() => [form.name, form.website, form.industry, form.description, form.brand_voice, form.locale, form.timezone], () => { if (!saving.value) { requestKey = crypto.randomUUID(); saved.value = false } })

function errorMessage(cause: unknown): string {
	const response = (cause as { response?: { data?: { msg?: unknown } } } | null)?.response
	if (response?.data?.msg) return String(response.data.msg)
	if (cause && typeof cause === 'object' && 'msg' in cause) return String(cause.msg)
	return cause instanceof Error ? cause.message : 'The business profile could not be saved.'
}
async function load() {
	loading.value = true; error.value = ''; saved.value = false
	try {
		const { profile } = await getBusinessProfile()
		if (profile) Object.assign(form, { name: profile.name || '', website: profile.website || '', industry: profile.industry || '', description: profile.description || '', brand_voice: profile.brand_voice || '', locale: profile.locale || 'en', timezone: profile.timezone || 'UTC', revision: profile.revision })
	} catch (cause) { error.value = errorMessage(cause) }
	finally { loading.value = false }
}
async function save() {
	if (saving.value || !canManage.value) return
	saving.value = true; error.value = ''; saved.value = false
	try {
		const profile = await saveBusinessProfile({ ...form, name: form.name.trim(), website: form.website.trim() }, requestKey)
		form.revision = profile.revision
		requestKey = crypto.randomUUID()
		saved.value = true
	} catch (cause) {
		const status = (cause as { response?: { status?: number } } | null)?.response?.status
		error.value = status === 409 ? 'The saved profile changed. Reload it before saving your edits again.' : errorMessage(cause)
	} finally { saving.value = false }
}
onMounted(load)
</script>
