<template>
	<div class="p-24px">
		<n-tabs v-model:value="activeTab" type="line" animated>
			<n-tab-pane name="send" tab="Send">
				<n-alert v-if="!config.enabled" type="warning" class="mb-16px">
					SMS sending is disabled. Configure a provider in the Settings tab first.
				</n-alert>
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Send to">
						<n-radio-group v-model:value="sendForm.target">
							<n-radio value="phone">One phone number (test)</n-radio>
							<n-radio value="group">Contact group</n-radio>
						</n-radio-group>
					</n-form-item>
					<n-form-item v-if="sendForm.target === 'phone'" label="Phone number">
						<n-input v-model:value="sendForm.phone" placeholder="03001234567 or +923001234567" />
					</n-form-item>
					<n-form-item
						v-else
						label="Contact group"
						feedback="Only subscribed contacts with a phone attribute are messaged.">
						<n-select
							v-model:value="sendForm.groupId"
							:options="groupOptions"
							placeholder="Select a group"
							filterable />
					</n-form-item>
					<n-form-item label="Message">
						<n-input
							v-model:value="sendForm.message"
							type="textarea"
							:rows="5"
							placeholder="Hi {name}, ..." />
					</n-form-item>
					<div class="mb-16px text-12px text-desc">
						{{ sendForm.message.length }} characters · {{ smsParts }} SMS per contact.
						<template v-if="sendForm.target === 'group'">
							Use {name}, {email} or any contact attribute as a placeholder.
						</template>
					</div>
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
								<n-empty description="No SMS sent yet" />
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
					<n-form-item label="Enable SMS sending">
						<n-switch v-model:value="config.enabled" />
					</n-form-item>
					<n-form-item label="Provider">
						<n-radio-group v-model:value="config.provider">
							<n-radio value="generic">HTTP gateway (local bulk SMS providers)</n-radio>
							<n-radio value="twilio">Twilio</n-radio>
						</n-radio-group>
					</n-form-item>
					<n-form-item
						label="Default country code"
						feedback="Local numbers like 0300... become +92300... with code 92.">
						<n-input v-model:value="config.default_country_code" placeholder="92" />
					</n-form-item>

					<template v-if="config.provider === 'twilio'">
						<n-form-item label="Account SID">
							<n-input
								v-model:value="config.twilio_account_sid"
								:input-props="{ autocomplete: 'off' }"
								placeholder="AC..." />
						</n-form-item>
						<n-form-item label="Auth token">
							<n-input
								v-model:value="config.twilio_auth_token"
								type="password"
								show-password-on="click"
								:input-props="{ autocomplete: 'new-password' }" />
						</n-form-item>
						<n-form-item label="From number">
							<n-input v-model:value="config.twilio_from" placeholder="+15550001111" />
						</n-form-item>
					</template>

					<template v-else>
						<n-alert type="info" class="mb-16px">
							Copy the API URL from your SMS provider and replace its values with placeholders:
							<code>{to}</code> (+923001234567), <code>{to_digits}</code> (923001234567),
							<code>{message}</code>, <code>{api_key}</code>, <code>{sender}</code>.
						</n-alert>
						<n-form-item label="Method">
							<n-radio-group v-model:value="config.generic_method">
								<n-radio value="GET">GET</n-radio>
								<n-radio value="POST">POST</n-radio>
							</n-radio-group>
						</n-form-item>
						<n-form-item label="API URL">
							<n-input
								v-model:value="config.generic_url"
								placeholder="https://provider.com/api/send?key={api_key}&to={to_digits}&mask={sender}&msg={message}" />
						</n-form-item>
						<template v-if="config.generic_method === 'POST'">
							<n-form-item label="Content type">
								<n-select v-model:value="config.generic_content_type" :options="contentTypeOptions" />
							</n-form-item>
							<n-form-item label="Request body">
								<n-input
									v-model:value="config.generic_body"
									type="textarea"
									:rows="4"
									placeholder='{"to":"{to}","message":"{message}","api_key":"{api_key}"}' />
							</n-form-item>
						</template>
						<n-form-item label="API key">
							<n-input
								v-model:value="config.generic_api_key"
								type="password"
								show-password-on="click"
								:input-props="{ autocomplete: 'new-password' }" />
						</n-form-item>
						<n-form-item label="Sender ID / mask" feedback="Brand name or number, if your provider uses one.">
							<n-input v-model:value="config.generic_sender" :input-props="{ autocomplete: 'off' }" />
						</n-form-item>
					</template>

					<n-button type="primary" @click="handleSaveConfig">Save</n-button>
				</n-form>
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
	getSmsConfig,
	saveSmsConfig,
	sendTestSms,
	sendGroupSms,
	getSmsLogs,
	type SmsConfig,
	type SmsLog,
} from '@/api/modules/sms'

const activeTab = ref('send')

// Settings
const config = ref<SmsConfig>({
	enabled: false,
	provider: 'generic',
	default_country_code: '92',
	twilio_account_sid: '',
	twilio_auth_token: '',
	twilio_from: '',
	generic_method: 'GET',
	generic_url: '',
	generic_body: '',
	generic_content_type: 'application/json',
	generic_api_key: '',
	generic_sender: '',
})

const contentTypeOptions = [
	{ label: 'application/json', value: 'application/json' },
	{ label: 'application/x-www-form-urlencoded', value: 'application/x-www-form-urlencoded' },
]

const loadConfig = async () => {
	const res = await getSmsConfig()
	if (isObject<SmsConfig>(res)) {
		config.value = { ...config.value, ...res }
	}
}

const handleSaveConfig = async () => {
	await saveSmsConfig(config.value)
	await loadConfig()
}

// Send
const sendForm = ref({
	target: 'phone' as 'phone' | 'group',
	phone: '',
	groupId: null as number | null,
	message: '',
})

const groupOptions = ref<SelectOption[]>([])

const loadGroups = async () => {
	const res = await getGroupAll()
	if (isObject<{ list: { id: number; name: string }[] | null }>(res)) {
		groupOptions.value = (res.list ?? []).map(item => ({ label: item.name, value: item.id }))
	}
}

// GSM messages split at 153 chars per part, Unicode (e.g. Urdu) at 67.
const smsParts = computed(() => {
	const msg = sendForm.value.message
	if (!msg) return 0
	const isUnicode = [...msg].some(ch => ch.charCodeAt(0) > 127)
	const single = isUnicode ? 70 : 160
	const multi = isUnicode ? 67 : 153
	return msg.length <= single ? 1 : Math.ceil(msg.length / multi)
})

const canSend = computed(() => {
	const { target, phone, groupId, message } = sendForm.value
	if (!config.value.enabled || !message.trim()) return false
	return target === 'phone' ? !!phone.trim() : !!groupId
})

const handleSend = async () => {
	const { target, phone, groupId, message } = sendForm.value
	try {
		if (target === 'phone') {
			await sendTestSms({ phone, message })
		} else {
			await sendGroupSms({ group_id: groupId!, message })
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

const { tableProps, pageProps, tableParams, fetchTable } = useDataTable<SmsLog>({
	params: {
		page: 1,
		page_size: 20,
		keyword: '',
		status: null as string | null,
	},
	fetchFn: getSmsLogs,
})

const columns: DataTableColumns<SmsLog> = [
	{ title: 'Phone', key: 'phone', width: 160 },
	{ title: 'Message', key: 'message', ellipsis: { tooltip: true } },
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
	{ title: 'Provider', key: 'provider', width: 100 },
	{ title: 'Time', key: 'create_time', width: 180, render: row => formatTime(row.create_time) },
]

onMounted(() => {
	loadConfig()
	loadGroups()
})
</script>
