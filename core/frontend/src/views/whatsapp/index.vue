<template>
	<div class="p-24px">
		<n-tabs v-model:value="activeTab" type="line" animated>
			<n-tab-pane name="send" tab="Send">
				<n-alert v-if="!config.enabled" type="warning" class="mb-16px">
					WhatsApp sending is disabled. Add your WhatsApp Cloud API details in the Settings tab first.
				</n-alert>
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Send to">
						<n-radio-group v-model:value="form.target">
							<n-radio value="phone">One phone number (test)</n-radio>
							<n-radio value="group">Contact group</n-radio>
						</n-radio-group>
					</n-form-item>
					<n-form-item v-if="form.target === 'phone'" label="Phone number">
						<n-input v-model:value="form.phone" placeholder="03001234567 or +923001234567" />
					</n-form-item>
					<n-form-item
						v-else
						label="Contact group"
						feedback="Only subscribed contacts with a phone attribute are messaged.">
						<n-select v-model:value="form.groupId" :options="groupOptions" placeholder="Select a group" filterable />
					</n-form-item>

					<n-form-item
						v-if="templateOptions.length"
						label="Template"
						feedback="Only templates approved by Meta can be sent.">
						<n-select
							:value="selectedTemplateKey"
							:options="templateOptions"
							placeholder="Select an approved template"
							filterable
							@update:value="handleSelectTemplate" />
					</n-form-item>
					<template v-else>
						<n-form-item label="Template name" feedback="Exact name of a template approved in WhatsApp Manager.">
							<n-input v-model:value="form.template" placeholder="appointment_reminder" />
						</n-form-item>
						<n-form-item label="Template language">
							<n-input v-model:value="form.language" placeholder="en" />
						</n-form-item>
					</template>

					<n-form-item :label="paramsLabel">
						<div class="w-full">
							<div v-for="(_, index) in form.params" :key="index" class="mb-8px flex items-center gap-8px">
								<span class="w-48px text-desc">{{ placeholderName(index) }}</span>
								<n-input v-model:value="form.params[index]" :placeholder="paramPlaceholder" />
								<n-button v-if="!selectedTemplate" text type="error" @click="form.params.splice(index, 1)">
									Remove
								</n-button>
							</div>
							<n-button v-if="!selectedTemplate" size="small" @click="form.params.push('')">Add value</n-button>
						</div>
					</n-form-item>

					<n-form-item v-if="preview" label="Preview">
						<div class="preview">{{ preview }}</div>
					</n-form-item>

					<n-button type="primary" :disabled="!canSend" @click="handleSend">Send</n-button>
				</n-form>
			</n-tab-pane>

			<n-tab-pane name="logs" tab="Logs">
				<bt-table-layout>
					<template #toolsRight>
						<n-input
							v-model:value="tableParams.keyword"
							placeholder="Search phone"
							clearable
							style="width: 200px; margin-right: 12px"
							@update:value="() => fetchTable(true)" />
						<n-select
							v-model:value="tableParams.status"
							:options="statusOptions"
							placeholder="Filter by status"
							clearable
							style="width: 160px"
							@update:value="() => fetchTable(true)" />
					</template>
					<template #table>
						<n-data-table v-bind="tableProps" :columns="columns">
							<template #empty>
								<n-empty description="No WhatsApp messages sent yet" />
							</template>
						</n-data-table>
					</template>
					<template #pageRight>
						<bt-table-page v-bind="pageProps" @refresh="fetchTable"> </bt-table-page>
					</template>
				</bt-table-layout>
			</n-tab-pane>

			<n-tab-pane name="settings" tab="Settings">
				<n-alert type="info" class="mb-16px max-w-640px">
					Uses the official WhatsApp Cloud API by Meta. Create an app at developers.facebook.com, add
					WhatsApp, then copy the IDs and a permanent access token here.
				</n-alert>
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Enable WhatsApp sending">
						<n-switch v-model:value="config.enabled" />
					</n-form-item>
					<n-form-item label="Phone number ID" feedback="WhatsApp Manager > API setup > Phone number ID">
						<n-input v-model:value="config.phone_number_id" :input-props="{ autocomplete: 'off' }" />
					</n-form-item>
					<n-form-item
						label="WhatsApp Business Account ID (optional)"
						feedback="Needed to list your approved templates on the Send tab.">
						<n-input v-model:value="config.business_account_id" :input-props="{ autocomplete: 'off' }" />
					</n-form-item>
					<n-form-item label="Access token">
						<n-input
							v-model:value="config.access_token"
							type="password"
							show-password-on="click"
							:input-props="{ autocomplete: 'new-password' }" />
					</n-form-item>
					<n-form-item label="Graph API version">
						<n-input v-model:value="config.api_version" :input-props="{ autocomplete: 'off' }" placeholder="v23.0" />
					</n-form-item>
					<n-form-item
						label="Default country code"
						feedback="Local numbers like 0300... become 92300... with code 92.">
						<n-input v-model:value="config.default_country_code" :input-props="{ autocomplete: 'off' }" />
					</n-form-item>
					<n-button type="primary" @click="handleSaveConfig">Save</n-button>
				</n-form>
			</n-tab-pane>
		</n-tabs>
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NTag, SelectOption } from 'naive-ui'
import { useDataTable } from '@/hooks/useDataTable'
import { isArray, isObject } from '@/utils'
import { formatTime } from '@/utils/time'
import { getGroupAll } from '@/api/modules/contacts/group'
import {
	getWhatsappConfig,
	saveWhatsappConfig,
	getWhatsappTemplates,
	sendWhatsappTest,
	sendWhatsappGroup,
	getWhatsappLogs,
	type WhatsappConfig,
	type WhatsappTemplate,
	type WhatsappLog,
} from '@/api/modules/whatsapp'

const activeTab = ref('send')

// Settings
const config = ref<WhatsappConfig>({
	enabled: false,
	phone_number_id: '',
	business_account_id: '',
	access_token: '',
	api_version: 'v23.0',
	default_country_code: '92',
})

const loadConfig = async () => {
	const res = await getWhatsappConfig()
	if (isObject<WhatsappConfig>(res)) {
		config.value = { ...config.value, ...res }
	}
}

const handleSaveConfig = async () => {
	await saveWhatsappConfig(config.value)
	await loadConfig()
	loadTemplates()
}

// Templates
const templates = ref<WhatsappTemplate[]>([])

const loadTemplates = async () => {
	if (!config.value.business_account_id || !config.value.access_token) return
	try {
		const res = await getWhatsappTemplates()
		templates.value = isArray<WhatsappTemplate>(res) ? res.filter(t => t.status === 'APPROVED') : []
	} catch {
		templates.value = []
	}
}

const templateKey = (t: { name: string; language: string }) => `${t.name}|${t.language}`

const templateOptions = computed<SelectOption[]>(() =>
	templates.value.map(t => ({ label: `${t.name} (${t.language})`, value: templateKey(t) }))
)

const selectedTemplateKey = computed(() => (form.value.template ? templateKey(form.value) : null))

const selectedTemplate = computed(() =>
	templates.value.find(t => templateKey(t) === selectedTemplateKey.value)
)

// Highest {{n}} in the template body = number of values the template needs
const countPlaceholders = (body: string) =>
	Math.max(0, ...[...body.matchAll(/\{\{(\d+)\}\}/g)].map(m => Number(m[1])))

const handleSelectTemplate = (key: string) => {
	const t = templates.value.find(item => templateKey(item) === key)
	if (!t) return
	form.value.template = t.name
	form.value.language = t.language
	form.value.params = Array.from({ length: countPlaceholders(t.body) }, () => '')
}

// Send
const form = ref({
	target: 'phone' as 'phone' | 'group',
	phone: '',
	groupId: null as number | null,
	template: '',
	language: 'en',
	params: [] as string[],
})

const placeholderName = (index: number) => `{{${index + 1}}}`

const paramsLabel = computed(() =>
	form.value.target === 'group'
		? 'Template values ({name}, {email} or any contact attribute are filled per contact)'
		: 'Template values'
)

const paramPlaceholder = computed(() => (form.value.target === 'group' ? '{name}' : 'Ali'))

const preview = computed(() => {
	const body = selectedTemplate.value?.body
	if (!body) return ''
	return body.replace(/\{\{(\d+)\}\}/g, (match, n) => form.value.params[Number(n) - 1] || match)
})

const groupOptions = ref<SelectOption[]>([])

const loadGroups = async () => {
	const res = await getGroupAll()
	if (isObject<{ list: { id: number; name: string }[] | null }>(res)) {
		groupOptions.value = (res.list ?? []).map(item => ({ label: item.name, value: item.id }))
	}
}

const canSend = computed(() => {
	const { target, phone, groupId, template, language, params } = form.value
	if (!config.value.enabled || !template.trim() || !language.trim()) return false
	if (params.some(p => !p.trim())) return false
	return target === 'phone' ? !!phone.trim() : !!groupId
})

const handleSend = async () => {
	const { target, phone, groupId, template, language, params } = form.value
	const fields = { template: template.trim(), language: language.trim(), params }
	try {
		if (target === 'phone') {
			await sendWhatsappTest({ ...fields, phone })
		} else {
			await sendWhatsappGroup({ ...fields, group_id: groupId! })
		}
	} finally {
		fetchTable()
	}
}

// Logs
const statusOptions = [
	{ label: 'Sent', value: 'sent' },
	{ label: 'Failed', value: 'failed' },
]

const { tableProps, pageProps, tableParams, fetchTable } = useDataTable<WhatsappLog>({
	params: {
		page: 1,
		page_size: 20,
		keyword: '',
		status: null as string | null,
	},
	fetchFn: getWhatsappLogs,
})

const columns: DataTableColumns<WhatsappLog> = [
	{ title: 'Phone', key: 'phone', width: 150 },
	{ title: 'Template', key: 'template', width: 180, ellipsis: { tooltip: true } },
	{ title: 'Values', key: 'params', ellipsis: { tooltip: true } },
	{
		title: 'Status',
		key: 'status',
		width: 100,
		render: row => (
			<NTag type={row.status === 'sent' ? 'success' : 'error'} size="small">
				{row.status}
			</NTag>
		),
	},
	{ title: 'Error', key: 'error', width: 240, ellipsis: { tooltip: true } },
	{ title: 'Time', key: 'create_time', width: 180, render: row => formatTime(row.create_time) },
]

onMounted(async () => {
	loadGroups()
	await loadConfig()
	loadTemplates()
})
</script>

<style lang="scss" scoped>
.preview {
	width: 100%;
	padding: 12px;
	white-space: pre-wrap;
	background: var(--color-fill-2, #f5f5f5);
	border-radius: 8px;
}
</style>
