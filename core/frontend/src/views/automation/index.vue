<template>
	<div class="p-24px">
		<div class="flex items-center mb-16px">
			<div class="flex-1">
				<h2 class="m-0 text-20px">Workflows</h2>
				<p class="m-0 mt-4px text-desc">
					Send emails, WhatsApp and SMS automatically when something happens to a contact.
				</p>
			</div>
			<n-button type="primary" @click="openNew()">
				<i class="i-mdi-plus mr-4px"></i>
				New workflow
			</n-button>
		</div>

		<div class="section-label">Start from a ready-made flow</div>
		<div class="preset-grid mb-24px">
			<div v-for="p in presets" :key="p.key" class="preset" @click="openNew(p.key)">
				<i :class="p.icon" class="text-24px"></i>
				<div class="preset-title">{{ p.title }}</div>
				<div class="preset-desc">{{ p.description }}</div>
			</div>
		</div>

		<div class="section-label">Your workflows</div>
		<n-data-table :columns="columns" :data="list" :loading="loading" :row-key="(r: Workflow) => r.id">
			<template #empty>
				<n-empty description="No workflows yet. Pick a ready-made flow above to start." />
			</template>
		</n-data-table>

		<workflow-activity ref="activityRef" v-model:show="activityShow" />
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NButton, NTag } from 'naive-ui'
import { confirm, isArray } from '@/utils'
import { deleteWorkflow, getWorkflowList, setWorkflowStatus, type Workflow } from '@/api/modules/workflow'
import { countSteps, describeStep, describeTrigger, presets } from './presets'
import { useWorkflowLookups } from './useWorkflowLookups'
import WorkflowActivity from './components/WorkflowActivity.vue'

const list = ref<Workflow[]>([])
const loading = ref(false)

const load = async () => {
	loading.value = true
	try {
		const res = await getWorkflowList()
		list.value = isArray<Workflow>(res) ? res : []
	} finally {
		loading.value = false
	}
}

// Names of groups, tags, templates and campaigns for the descriptions
const { lookups, load: loadLookups } = useWorkflowLookups()

// The editor is its own page; activity opens in a drawer
const router = useRouter()
const activityRef = ref<InstanceType<typeof WorkflowActivity>>()
const activityShow = ref(false)

const openNew = (preset?: string) => {
	router.push(preset ? `/automation/edit?preset=${preset}` : '/automation/edit')
}

const openEdit = (w: Workflow) => {
	router.push(`/automation/edit?id=${w.id}`)
}

const toggle = (w: Workflow) => {
	const turnOn = w.status !== 'active'
	confirm({
		title: turnOn ? `Turn on "${w.name}"?` : `Pause "${w.name}"?`,
		content: turnOn
			? 'Contacts will start entering and messages will be sent automatically.'
			: 'No new contacts will enter and waiting steps will hold until you turn it on again.',
		onConfirm: async () => {
			await setWorkflowStatus(w.id, turnOn ? 'active' : 'paused')
			load()
		},
	})
}

const remove = (w: Workflow) => {
	confirm({
		title: `Delete "${w.name}"?`,
		content: 'Its history and contacts in progress are removed too. This cannot be undone.',
		onConfirm: async () => {
			await deleteWorkflow(w.id)
			load()
		},
	})
}

const statusTag = {
	draft: { type: 'default', text: 'Draft' },
	active: { type: 'success', text: 'On' },
	paused: { type: 'warning', text: 'Paused' },
} as const

const columns: DataTableColumns<Workflow> = [
	{
		title: 'Workflow',
		key: 'name',
		minWidth: 260,
		render: row => (
			<div>
				<div class="font-600">{row.name}</div>
				<div class="text-desc text-12px">
					<i class="i-mdi-lightning-bolt-outline mr-2px align-middle"></i>
					{describeTrigger(row.trigger, lookups)}
				</div>
			</div>
		),
	},
	{
		title: 'Steps',
		key: 'steps',
		minWidth: 260,
		render: row => (
			<div class="text-12px">
				{row.steps.slice(0, 4).map((s, i) => (
					<div key={i} class="truncate max-w-360px">
						{i + 1}. {describeStep(s, lookups)}
					</div>
				))}
				{countSteps(row.steps) > row.steps.slice(0, 4).length ? (
					<div class="text-desc">+{countSteps(row.steps) - row.steps.slice(0, 4).length} more (incl. branches)</div>
				) : null}
			</div>
		),
	},
	{
		title: 'Status',
		key: 'status',
		width: 90,
		render: row => (
			<NTag type={statusTag[row.status].type} size="small">
				{statusTag[row.status].text}
			</NTag>
		),
	},
	{
		title: 'Contacts',
		key: 'stats',
		width: 170,
		render: row => (
			<div class="text-12px">
				<div>In progress: {row.stats?.active ?? 0}</div>
				<div>Finished: {row.stats?.completed ?? 0}</div>
				<div>Stopped: {row.stats?.exited ?? 0}</div>
				{row.stats?.skipped ? <div class="text-desc">Didn't start: {row.stats.skipped}</div> : null}
			</div>
		),
	},
	{
		title: 'Actions',
		key: 'actions',
		width: 260,
		render: row => (
			<div class="flex flex-wrap gap-8px">
				<NButton size="small" type={row.status === 'active' ? 'warning' : 'primary'} onClick={() => toggle(row)}>
					{row.status === 'active' ? 'Pause' : 'Turn on'}
				</NButton>
				<NButton size="small" onClick={() => openEdit(row)}>
					Edit
				</NButton>
				<NButton size="small" onClick={() => activityRef.value?.open(row)}>
					Activity
				</NButton>
				<NButton size="small" type="error" quaternary onClick={() => remove(row)}>
					Delete
				</NButton>
			</div>
		),
	},
]

onMounted(() => {
	load()
	loadLookups()
})
</script>

<style lang="scss" scoped>
.text-desc {
	color: var(--color-text-3, #888);
}

.section-label {
	margin-bottom: 10px;
	font-weight: 600;
}

.preset-grid {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
	gap: 12px;
}

.preset {
	padding: 14px;
	cursor: pointer;
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 8px;
	transition:
		border-color 0.2s,
		box-shadow 0.2s;

	&:hover {
		border-color: var(--color-primary-1, #18a058);
		box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
	}
}

.preset-title {
	margin: 6px 0 4px;
	font-weight: 600;
}

.preset-desc {
	font-size: 12px;
	line-height: 1.5;
	color: var(--color-text-3, #888);
}
</style>
