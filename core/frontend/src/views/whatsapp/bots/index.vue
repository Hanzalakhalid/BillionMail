<template>
	<div class="p-24px">
		<div class="page-head">
			<div>
				<h2>WhatsApp Bots</h2>
				<p>Build and publish a separate automated menu for each company and WhatsApp number.</p>
			</div>
			<n-button
				type="primary"
				:disabled="!channels.length"
				@click="router.push('/whatsapp/bots/edit')">
				<i class="i-mdi-plus mr-6px" />Create bot
			</n-button>
		</div>

		<n-alert v-if="!channels.length" type="warning" class="mb-16px">
			Connect a company WhatsApp number first. Open <b>WhatsApp → Settings</b> and add the Meta
			Cloud API channel.
		</n-alert>
		<n-data-table
			:columns="columns"
			:data="bots"
			:loading="loading"
			:row-key="(row: ChatbotFlow) => row.id">
			<template #empty><n-empty description="No company bots yet" /></template>
		</n-data-table>
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NButton, NTag } from 'naive-ui'
import {
	listChatbots,
	deleteChatbot,
	publishChatbot,
	type ChatbotFlow,
} from '@/api/modules/chatbot'
import { getWhatsappChannels, type WhatsappChannel } from '@/api/modules/whatsapp'
import { confirm, isArray } from '@/utils'

const router = useRouter()
const bots = ref<ChatbotFlow[]>([])
const channels = ref<WhatsappChannel[]>([])
const loading = ref(false)
const load = async () => {
	loading.value = true
	try {
		const [botData, channelData] = await Promise.all([listChatbots(), getWhatsappChannels()])
		bots.value = isArray<ChatbotFlow>(botData) ? botData : []
		channels.value = isArray<WhatsappChannel>(channelData) ? channelData : []
	} finally {
		loading.value = false
	}
}
const publish = (row: ChatbotFlow) =>
	confirm({
		title: `Publish “${row.name}”?`,
		content:
			'This version becomes the live bot for this WhatsApp number. Any other live bot on the number is paused.',
		onConfirm: async () => {
			await publishChatbot(row.id)
			load()
		},
	})
const remove = (row: ChatbotFlow) =>
	confirm({
		title: `Delete “${row.name}”?`,
		content: 'The bot is removed from this workspace. Published history is retained in audit logs.',
		onConfirm: async () => {
			await deleteChatbot(row.id)
			load()
		},
	})
const columns: DataTableColumns<ChatbotFlow> = [
	{
		title: 'Bot',
		key: 'name',
		render: row => (
			<div>
				<b>{row.name}</b>
				<div class="muted">
					{row.verified_name || row.display_phone_number || 'WhatsApp number'}
				</div>
			</div>
		),
	},
	{ title: 'Number', key: 'display_phone_number', render: row => row.display_phone_number || '—' },
	{
		title: 'Version',
		key: 'version',
		render: row =>
			row.draft_version ? `Draft v${row.draft_version}` : `Published v${row.published_version}`,
	},
	{
		title: 'Status',
		key: 'status',
		render: row => (
			<NTag
				type={
					row.status === 'active' ? 'success' : row.status === 'paused' ? 'warning' : 'default'
				}>
				{row.status}
			</NTag>
		),
	},
	{
		title: 'Actions',
		key: 'actions',
		width: 280,
		render: row => (
			<div class="flex gap-8px">
				<NButton size="small" onClick={() => router.push(`/whatsapp/bots/edit?id=${row.id}`)}>
					Edit
				</NButton>
				<NButton
					size="small"
					type="primary"
					disabled={!row.draft_version}
					onClick={() => publish(row)}>
					Publish
				</NButton>
				<NButton size="small" type="error" quaternary onClick={() => remove(row)}>
					Delete
				</NButton>
			</div>
		),
	},
]
onMounted(load)
</script>

<style scoped lang="scss">
.page-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 20px;
	h2 {
		margin: 0;
	}
	p {
		margin: 5px 0 0;
		color: #8b8f97;
	}
}
.muted {
	margin-top: 3px;
	font-size: 12px;
	color: #8b8f97;
}
</style>
