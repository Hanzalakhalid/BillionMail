<template>
	<div class="p-24px">
		<n-tabs v-model:value="activeTab" type="line" animated>
			<n-tab-pane name="send" tab="Send">
				<n-alert v-if="!config.enabled" type="warning" class="mb-16px">
					Push notifications are disabled. Add your OneSignal keys in the Settings tab first.
				</n-alert>
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Send to">
						<n-radio-group v-model:value="form.audience">
							<n-radio value="all">All subscribers</n-radio>
							<n-radio value="group">Contact group</n-radio>
						</n-radio-group>
					</n-form-item>
					<n-form-item
						v-if="form.audience === 'group'"
						label="Contact group"
						feedback="Only contacts your website identified with OneSignal.login(email) receive it.">
						<n-select v-model:value="form.groupId" :options="groupOptions" placeholder="Select a group" filterable />
					</n-form-item>
					<n-form-item label="Title">
						<n-input v-model:value="form.title" maxlength="100" show-count placeholder="Big sale today!" />
					</n-form-item>
					<n-form-item label="Message">
						<n-input
							v-model:value="form.message"
							type="textarea"
							:rows="4"
							maxlength="500"
							show-count
							placeholder="50% off on all plans. Tap to see offers." />
					</n-form-item>
					<n-form-item label="Link (opened on click)">
						<n-input v-model:value="form.url" placeholder="https://yourwebsite.com/offer" />
					</n-form-item>
					<n-form-item label="Image URL (optional)">
						<n-input v-model:value="form.image" placeholder="https://yourwebsite.com/banner.png" />
					</n-form-item>
					<n-button type="primary" :disabled="!canSend" @click="handleSend">Send</n-button>
				</n-form>
			</n-tab-pane>

			<n-tab-pane name="logs" tab="Logs">
				<bt-table-layout>
					<template #toolsRight>
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
								<n-empty description="No push notifications sent yet" />
							</template>
						</n-data-table>
					</template>
					<template #pageRight>
						<bt-table-page v-bind="pageProps" @refresh="fetchTable"> </bt-table-page>
					</template>
				</bt-table-layout>
			</n-tab-pane>

			<n-tab-pane name="settings" tab="Settings">
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Enable push notifications">
						<n-switch v-model:value="config.enabled" />
					</n-form-item>
					<n-form-item label="OneSignal App ID" feedback="OneSignal → Settings → Keys & IDs">
						<n-input
							v-model:value="config.app_id"
							:input-props="{ autocomplete: 'off' }"
							placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
					</n-form-item>
					<n-form-item label="OneSignal REST API key">
						<n-input
							v-model:value="config.rest_api_key"
							type="password"
							show-password-on="click"
							:input-props="{ autocomplete: 'new-password' }" />
					</n-form-item>
					<n-form-item label="Default link" feedback="Opened on click when a notification has no link.">
						<n-input v-model:value="config.default_url" placeholder="https://yourwebsite.com" />
					</n-form-item>
					<n-button type="primary" @click="handleSaveConfig">Save</n-button>
				</n-form>

				<div class="mt-32px max-w-760px">
					<div class="mb-8px font-bold">Add this to your website</div>
					<div class="mb-8px text-12px text-desc">
						Paste it before &lt;/head&gt;. Visitors who click "Allow" become subscribers. Call
						OneSignal.login with the visitor's email after they sign up so contact group sends can
						reach them.
					</div>
					<pre class="snippet">{{ websiteSnippet }}</pre>
				</div>
			</n-tab-pane>
		</n-tabs>
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NTag, SelectOption } from 'naive-ui'
import { useDataTable } from '@/hooks/useDataTable'
import { isObject } from '@/utils'
import { formatTime } from '@/utils/time'
import { getGroupAll } from '@/api/modules/contacts/group'
import {
	getPushConfig,
	savePushConfig,
	sendPush,
	getPushLogs,
	type PushConfig,
	type PushLog,
} from '@/api/modules/push'

const activeTab = ref('send')

// Settings
const config = ref<PushConfig>({
	enabled: false,
	app_id: '',
	rest_api_key: '',
	default_url: '',
})

const loadConfig = async () => {
	const res = await getPushConfig()
	if (isObject<PushConfig>(res)) {
		config.value = { ...config.value, ...res }
	}
}

const handleSaveConfig = async () => {
	await savePushConfig(config.value)
	await loadConfig()
}

// Script tags are assembled from pieces so the SFC parser does not treat them as markup
const tag = 'script'
const websiteSnippet = computed(() =>
	[
		`<${tag} src="https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js" defer></${tag}>`,
		`<${tag}>`,
		'  window.OneSignalDeferred = window.OneSignalDeferred || [];',
		'  OneSignalDeferred.push(async function (OneSignal) {',
		`    await OneSignal.init({ appId: "${config.value.app_id || 'YOUR_APP_ID'}" });`,
		'    // After a visitor signs up or logs in (lowercase email):',
		'    // await OneSignal.login("customer@example.com");',
		'  });',
		`</${tag}>`,
	].join('\n')
)

// Send
const form = ref({
	audience: 'all' as 'all' | 'group',
	groupId: null as number | null,
	title: '',
	message: '',
	url: '',
	image: '',
})

const groupOptions = ref<SelectOption[]>([])

const loadGroups = async () => {
	const res = await getGroupAll()
	if (isObject<{ list: { id: number; name: string }[] | null }>(res)) {
		groupOptions.value = (res.list ?? []).map(item => ({ label: item.name, value: item.id }))
	}
}

const canSend = computed(() => {
	const { audience, groupId, message } = form.value
	if (!config.value.enabled || !message.trim()) return false
	return audience === 'all' || !!groupId
})

const handleSend = async () => {
	const { audience, groupId, title, message, url, image } = form.value
	try {
		await sendPush({
			audience,
			group_id: audience === 'group' ? groupId! : undefined,
			title,
			message,
			url: url.trim(),
			image: image.trim(),
		})
	} finally {
		fetchTable()
	}
}

// Logs
const statusOptions = [
	{ label: 'Sent', value: 'sent' },
	{ label: 'Failed', value: 'failed' },
]

const { tableProps, pageProps, tableParams, fetchTable } = useDataTable<PushLog>({
	params: {
		page: 1,
		page_size: 20,
		status: null as string | null,
	},
	fetchFn: getPushLogs,
})

const columns: DataTableColumns<PushLog> = [
	{ title: 'Title', key: 'title', width: 180, ellipsis: { tooltip: true } },
	{ title: 'Message', key: 'message', ellipsis: { tooltip: true } },
	{
		title: 'Audience',
		key: 'audience',
		width: 140,
		render: row => (row.audience === 'all' ? 'All subscribers' : `${row.audience} (${row.targeted})`),
	},
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

onMounted(() => {
	loadConfig()
	loadGroups()
})
</script>

<style lang="scss" scoped>
.snippet {
	padding: 12px;
	overflow-x: auto;
	font-size: 12px;
	line-height: 1.6;
	white-space: pre;
	background: var(--color-fill-2, #f5f5f5);
	border-radius: 4px;
}
</style>
