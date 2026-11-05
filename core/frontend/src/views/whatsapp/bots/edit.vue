<template>
	<div class="bot-builder">
		<header class="builder-topbar">
			<n-button quaternary circle @click="router.push('/whatsapp/bots')"
				><i class="i-mdi-arrow-left text-20px"
			/></n-button>
			<div class="flex-1 min-w-0">
				<input v-model="form.name" class="name-input" placeholder="Customer support bot" />
				<div class="sub">Company-scoped WhatsApp menu flow</div>
			</div>
			<span v-if="problem" class="problem"
				><i class="i-mdi-alert-circle-outline" /> {{ problem }}</span
			>
			<n-button @click="save(false)">Save draft</n-button>
			<n-button type="primary" :disabled="!!problem" @click="save(true)"
				><i class="i-mdi-rocket-launch-outline mr-5px" />Publish</n-button
			>
		</header>

		<div class="builder-body">
			<aside class="settings-panel">
				<h3>Bot settings</h3>
				<label>WhatsApp number</label>
				<n-select
					v-model:value="form.phone_number_id"
					:options="channelOptions"
					placeholder="Select company number" />
				<label>Welcome message</label>
				<n-input
					v-model:value="form.welcome"
					type="textarea"
					:rows="4"
					maxlength="4096"
					show-count />
				<label>Menu question</label>
				<n-input
					v-model:value="form.prompt"
					type="textarea"
					:rows="3"
					placeholder="Please select an option:" />
				<n-alert type="info" class="mt-16px"
					>Each workspace sees and publishes only its own channels and bots. Published versions stay
					immutable.</n-alert
				>
			</aside>

			<main class="flow-canvas">
				<div class="canvas-title">
					<div><b>Conversation flow</b><span>Incoming message starts this bot</span></div>
					<n-button size="small" :disabled="form.options.length >= 10" @click="addOption"
						><i class="i-mdi-plus mr-4px" />Add menu option</n-button
					>
				</div>
				<div class="flow-column">
					<div class="flow-node trigger">
						<div class="node-icon"><i class="i-mdi-whatsapp" /></div>
						<div><small>TRIGGER</small><b>Customer sends a message</b></div>
					</div>
					<div class="connector" />
					<div class="flow-node">
						<div class="node-icon message"><i class="i-mdi-message-text-outline" /></div>
						<div>
							<small>MESSAGE</small><b>{{ form.welcome || 'Welcome message' }}</b>
						</div>
					</div>
					<div class="connector" />
					<div class="flow-node menu">
						<div class="node-icon menu-icon"><i class="i-mdi-format-list-numbered" /></div>
						<div class="flex-1">
							<small>MENU · WAITS FOR REPLY</small><b>{{ form.prompt || 'Menu question' }}</b>
							<div v-for="(option, index) in form.options" :key="option.id" class="menu-row">
								<span>{{ index + 1 }}</span
								>{{ option.label || 'Untitled option' }}
							</div>
						</div>
					</div>
					<div class="branches">
						<div v-for="(option, index) in form.options" :key="option.id" class="branch-card">
							<div class="branch-head">
								<span>{{ index + 1 }}</span
								><b>Option branch</b
								><n-button
									text
									type="error"
									:disabled="form.options.length === 1"
									@click="removeOption(index)"
									><i class="i-mdi-delete-outline"
								/></n-button>
							</div>
							<n-input v-model:value="option.label" placeholder="Products" class="mb-8px" />
							<n-input v-model:value="option.value" placeholder="products" class="mb-8px" />
							<n-input
								v-model:value="option.reply"
								type="textarea"
								:rows="4"
								placeholder="Reply sent for this option" />
							<div class="end-label"><i class="i-mdi-flag-checkered" /> End conversation</div>
						</div>
					</div>
				</div>
			</main>

			<aside class="preview-panel">
				<div class="preview-title"><i class="i-mdi-cellphone-message" /> Test preview</div>
				<div class="phone">
					<div class="phone-head">
						<i class="i-mdi-whatsapp" />
						<div>
							<b>{{ selectedChannel?.verified_name || 'Your business' }}</b
							><span>{{ selectedChannel?.display_phone_number || 'WhatsApp Bot' }}</span>
						</div>
					</div>
					<div class="chat">
						<div class="bubble customer">Hi</div>
						<div class="bubble bot">{{ form.welcome || 'Your welcome message' }}</div>
						<div class="bubble bot">
							<div>{{ form.prompt || 'Please select an option:' }}</div>
							<div
								v-for="(option, index) in form.options"
								:key="option.id"
								class="preview-option"
								@click="previewReply = option.reply">
								<b>{{ index + 1 }}.</b> {{ option.label || 'Option' }}
							</div>
						</div>
						<div v-if="previewReply" class="bubble customer">1</div>
						<div v-if="previewReply" class="bubble bot">{{ previewReply }}</div>
					</div>
					<div class="phone-input">Type a message <i class="i-mdi-send" /></div>
				</div>
				<p class="preview-hint">Click a menu option to preview its reply.</p>
			</aside>
		</div>
	</div>
</template>

<script setup lang="ts">
import {
	createChatbot,
	getChatbot,
	publishChatbot,
	updateChatbot,
	type BotGraph,
	type BotNode,
	type ChatbotFlow,
} from '@/api/modules/chatbot'
import { getWhatsappChannels, type WhatsappChannel } from '@/api/modules/whatsapp'
import { isArray, isObject, Message } from '@/utils'

interface OptionForm {
	id: string
	label: string
	value: string
	reply: string
}
const route = useRoute()
const router = useRouter()
const editID = ref(String(route.query.id || ''))
const channels = ref<WhatsappChannel[]>([])
const previewReply = ref('')
const form = ref({
	name: 'Welcome & Main Menu',
	phone_number_id: '',
	welcome: 'Welcome! How can we help you today?',
	prompt: 'Please select an option:',
	options: [
		{
			id: 'products',
			label: 'Products',
			value: 'products',
			reply: 'Here is our product information.',
		},
		{
			id: 'support',
			label: 'Talk to Support',
			value: 'support',
			reply: 'A support agent will assist you shortly.',
		},
	] as OptionForm[],
})

const channelOptions = computed(() =>
	channels.value.map(item => ({
		label: `${item.verified_name || item.business_name || 'WhatsApp'} · ${item.display_phone_number || item.phone_number_id}`,
		value: item.id,
	}))
)
const selectedChannel = computed(() =>
	channels.value.find(item => item.id === form.value.phone_number_id)
)
const problem = computed(() => {
	if (!form.value.name.trim()) return 'Enter a bot name'
	if (!form.value.phone_number_id) return 'Select a WhatsApp number'
	if (!form.value.welcome.trim()) return 'Enter a welcome message'
	if (!form.value.prompt.trim()) return 'Enter a menu question'
	if (!form.value.options.length) return 'Add at least one option'
	const labels = new Set<string>()
	const values = new Set<string>()
	for (const option of form.value.options) {
		if (!option.label.trim() || !option.value.trim() || !option.reply.trim())
			return 'Complete every menu option and reply'
		if (
			labels.has(option.label.trim().toLowerCase()) ||
			values.has(option.value.trim().toLowerCase())
		)
			return 'Menu labels and values must be unique'
		labels.add(option.label.trim().toLowerCase())
		values.add(option.value.trim().toLowerCase())
	}
	return ''
})
const safeID = (value: string, index: number) =>
	value
		.toLowerCase()
		.replace(/[^a-z0-9_-]/g, '-')
		.replace(/-+/g, '-')
		.replace(/^-|-$/g, '') || `option-${index + 1}`
const graph = (): BotGraph => {
	const options = form.value.options.map((option, index) => ({
		...option,
		id: safeID(option.id || option.value, index),
	}))
	const nodes: BotNode[] = [
		{ id: 'trigger', type: 'trigger', position: { x: 320, y: 20 }, data: {} },
		{
			id: 'welcome',
			type: 'message',
			position: { x: 320, y: 150 },
			data: { text: form.value.welcome.trim() },
		},
		{
			id: 'main-menu',
			type: 'menu',
			position: { x: 320, y: 300 },
			data: {
				text: form.value.prompt.trim(),
				options: options.map(({ id, label, value }) => ({
					id,
					label: label.trim(),
					value: value.trim(),
				})),
			},
		},
	]
	const edges = [
		{ id: 'trigger-welcome', source: 'trigger', target: 'welcome' },
		{ id: 'welcome-menu', source: 'welcome', target: 'main-menu' },
	]
	options.forEach((option, index) => {
		const replyID = `reply-${option.id}`,
			endID = `end-${option.id}`
		nodes.push(
			{
				id: replyID,
				type: 'message',
				position: { x: 80 + index * 250, y: 500 },
				data: { text: option.reply.trim() },
			},
			{ id: endID, type: 'end', position: { x: 80 + index * 250, y: 650 }, data: {} }
		)
		edges.push(
			{ id: `menu-${option.id}`, source: 'main-menu', target: replyID, source_handle: option.id },
			{ id: `${replyID}-end`, source: replyID, target: endID }
		)
	})
	return { schema_version: 1, nodes, edges }
}
const hydrate = (flow: ChatbotFlow) => {
	form.value.name = flow.name
	form.value.phone_number_id = flow.phone_number_id
	const welcome = flow.graph.nodes.find(node => node.id === 'welcome')
	const menu = flow.graph.nodes.find(node => node.type === 'menu')
	form.value.welcome = welcome?.data.text || ''
	form.value.prompt = menu?.data.text || ''
	form.value.options = (menu?.data.options || []).map(option => {
		const target = flow.graph.edges.find(
			edge => edge.source === menu?.id && edge.source_handle === option.id
		)?.target
		const reply = flow.graph.nodes.find(node => node.id === target)?.data.text || ''
		return { ...option, reply }
	})
}
const addOption = () => {
	const id = `option-${Date.now()}`
	form.value.options.push({ id, label: '', value: '', reply: '' })
}
const removeOption = (index: number) => form.value.options.splice(index, 1)
const save = async (publish: boolean) => {
	if (problem.value) {
		Message.error(problem.value)
		return
	}
	const payload = {
		name: form.value.name.trim(),
		phone_number_id: form.value.phone_number_id,
		graph: graph(),
	}
	if (editID.value) await updateChatbot(editID.value, payload)
	else {
		const created = await createChatbot(payload)
		if (isObject<{ id: string }>(created)) editID.value = created.id
	}
	if (publish && editID.value) {
		await publishChatbot(editID.value)
		router.push('/whatsapp/bots')
	} else if (!editID.value) router.push('/whatsapp/bots')
}
onMounted(async () => {
	const channelData = await getWhatsappChannels()
	channels.value = isArray<WhatsappChannel>(channelData) ? channelData : []
	if (!form.value.phone_number_id && channels.value.length)
		form.value.phone_number_id = channels.value[0].id
	if (editID.value) {
		const data = await getChatbot(editID.value)
		if (isObject<ChatbotFlow>(data)) hydrate(data)
	}
})
</script>

<style scoped lang="scss">
.bot-builder {
	height: calc(100vh - 64px);
	min-height: 680px;
	background: #f4f6f8;
}
.builder-topbar {
	height: 72px;
	display: flex;
	align-items: center;
	gap: 12px;
	padding: 0 18px;
	background: white;
	border-bottom: 1px solid #e5e7eb;
}
.name-input {
	width: min(520px, 100%);
	padding: 0;
	font-size: 18px;
	font-weight: 700;
	border: 0;
	outline: none;
}
.sub {
	margin-top: 2px;
	font-size: 11px;
	color: #909399;
}
.problem {
	max-width: 280px;
	color: #d03050;
	font-size: 12px;
}
.builder-body {
	display: grid;
	grid-template-columns: 280px minmax(480px, 1fr) 330px;
	height: calc(100% - 73px);
}
.settings-panel,
.preview-panel {
	padding: 20px;
	overflow: auto;
	background: white;
}
.settings-panel {
	border-right: 1px solid #e5e7eb;
}
.preview-panel {
	border-left: 1px solid #e5e7eb;
}
.settings-panel h3 {
	margin: 0 0 20px;
}
.settings-panel label {
	display: block;
	margin: 16px 0 7px;
	font-size: 12px;
	font-weight: 700;
}
.flow-canvas {
	padding: 20px 24px 60px;
	overflow: auto;
	background-image: radial-gradient(#d7dce2 1px, transparent 1px);
	background-size: 20px 20px;
}
.canvas-title {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 12px 16px;
	background: rgb(255 255 255 / 90%);
	border: 1px solid #e5e7eb;
	border-radius: 10px;
}
.canvas-title span {
	margin-left: 10px;
	font-size: 12px;
	color: #909399;
}
.flow-column {
	display: flex;
	flex-direction: column;
	align-items: center;
	margin-top: 28px;
}
.flow-node {
	display: flex;
	gap: 12px;
	align-items: center;
	width: 360px;
	padding: 14px;
	background: white;
	border: 1px solid #dfe3e8;
	border-radius: 12px;
	box-shadow: 0 4px 15px rgb(30 41 59 / 7%);
}
.flow-node small {
	display: block;
	color: #909399;
	font-size: 10px;
	font-weight: 700;
}
.flow-node b {
	display: block;
	max-width: 290px;
	margin-top: 3px;
	font-size: 13px;
	white-space: pre-wrap;
}
.flow-node.trigger {
	border-color: #22c55e;
}
.flow-node.menu {
	align-items: flex-start;
	border-color: #8b5cf6;
}
.node-icon {
	display: grid;
	flex: 0 0 38px;
	height: 38px;
	place-items: center;
	color: white;
	background: #22c55e;
	border-radius: 10px;
}
.node-icon.message {
	background: #3b82f6;
}
.node-icon.menu-icon {
	background: #8b5cf6;
}
.connector {
	width: 2px;
	height: 28px;
	background: #cbd5e1;
}
.menu-row {
	display: flex;
	gap: 7px;
	align-items: center;
	margin-top: 7px;
	padding: 6px 8px;
	font-size: 12px;
	background: #f5f3ff;
	border-radius: 6px;
}
.menu-row span,
.branch-head span {
	display: grid;
	width: 20px;
	height: 20px;
	place-items: center;
	color: white;
	background: #8b5cf6;
	border-radius: 50%;
}
.branches {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
	gap: 14px;
	align-self: stretch;
	margin-top: 30px;
}
.branch-card {
	padding: 14px;
	background: white;
	border: 1px solid #dfe3e8;
	border-top: 3px solid #8b5cf6;
	border-radius: 10px;
}
.branch-head {
	display: flex;
	gap: 8px;
	align-items: center;
	margin-bottom: 12px;
}
.branch-head b {
	flex: 1;
	font-size: 12px;
}
.end-label {
	margin-top: 12px;
	color: #909399;
	font-size: 11px;
	text-align: center;
}
.preview-title {
	display: flex;
	gap: 8px;
	align-items: center;
	margin-bottom: 14px;
	font-weight: 700;
}
.phone {
	overflow: hidden;
	border: 7px solid #1f2937;
	border-radius: 24px;
	box-shadow: 0 10px 30px rgb(15 23 42 / 18%);
}
.phone-head {
	display: flex;
	gap: 9px;
	align-items: center;
	padding: 12px;
	color: white;
	background: #075e54;
}
.phone-head > i {
	font-size: 25px;
}
.phone-head b,
.phone-head span {
	display: block;
}
.phone-head span {
	font-size: 10px;
	opacity: 0.8;
}
.chat {
	min-height: 410px;
	padding: 12px;
	background: #efeae2;
}
.bubble {
	width: fit-content;
	max-width: 90%;
	margin-bottom: 8px;
	padding: 8px 10px;
	white-space: pre-wrap;
	background: white;
	border-radius: 8px;
	box-shadow: 0 1px 2px rgb(0 0 0 / 10%);
	font-size: 12px;
}
.bubble.customer {
	margin-left: auto;
	background: #d9fdd3;
}
.preview-option {
	margin-top: 6px;
	padding: 6px;
	cursor: pointer;
	color: #087f5b;
	border-top: 1px solid #e5e7eb;
}
.phone-input {
	display: flex;
	justify-content: space-between;
	padding: 12px;
	color: #9ca3af;
	background: white;
	font-size: 12px;
}
.preview-hint {
	color: #909399;
	font-size: 11px;
	text-align: center;
}
@media (max-width: 1150px) {
	.builder-body {
		grid-template-columns: 250px 1fr;
	}
	.preview-panel {
		display: none;
	}
}
</style>
