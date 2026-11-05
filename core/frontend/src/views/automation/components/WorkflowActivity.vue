<template>
	<n-drawer v-model:show="show" :width="860" placement="right">
		<n-drawer-content :title="workflow ? `${workflow.name}: activity` : 'Activity'" closable>
			<n-tabs v-model:value="tab" type="line" animated>
				<n-tab-pane name="runs" tab="Contacts">
					<div class="flex gap-12px mb-12px">
						<n-input
							v-model:value="runs.tableParams.value.keyword"
							placeholder="Search email"
							clearable
							class="w-220px"
							@update:value="() => runs.fetchTable(true)" />
						<n-select
							v-model:value="runs.tableParams.value.status"
							:options="runStatusOptions"
							placeholder="All statuses"
							clearable
							class="w-180px"
							@update:value="() => runs.fetchTable(true)" />
					</div>
					<n-data-table v-bind="runs.tableProps.value" :columns="runColumns">
						<template #empty>
							<n-empty description="No contacts have entered yet" />
						</template>
					</n-data-table>
					<div class="flex justify-end mt-12px">
						<bt-table-page v-bind="runs.pageProps.value" @refresh="runs.fetchTable" />
					</div>
				</n-tab-pane>

				<n-tab-pane name="logs" tab="What happened">
					<div class="flex gap-12px mb-12px">
						<n-select
							v-model:value="logs.tableParams.value.status"
							:options="logStatusOptions"
							placeholder="All results"
							clearable
							class="w-180px"
							@update:value="() => logs.fetchTable(true)" />
					</div>
					<n-data-table v-bind="logs.tableProps.value" :columns="logColumns">
						<template #empty>
							<n-empty description="Nothing has run yet" />
						</template>
					</n-data-table>
					<div class="flex justify-end mt-12px">
						<bt-table-page v-bind="logs.pageProps.value" @refresh="logs.fetchTable" />
					</div>
				</n-tab-pane>

				<n-tab-pane name="reviews" :tab="pendingCount ? `Needs review (${pendingCount})` : 'Needs review'">
					<div class="flex flex-wrap items-center gap-12px mb-12px">
						<n-select
							v-model:value="reviews.tableParams.value.status"
							:options="reviewStatusOptions"
							class="w-180px"
							@update:value="() => reviews.fetchTable(true)" />
						<template v-if="reviews.tableParams.value.status === 'pending'">
							<n-button type="primary" size="small" :disabled="!checked.length" @click="resolve('send')">
								Send selected ({{ checked.length }})
							</n-button>
							<n-button size="small" :disabled="!checked.length" @click="resolve('discard')">Discard selected</n-button>
						</template>
						<span class="text-12px text-[#888]">
							Manual messages wait here. Sending uses the step as it is now in the workflow.
						</span>
					</div>
					<n-data-table
						v-bind="reviews.tableProps.value"
						:columns="reviewColumns">
						<template #empty>
							<n-empty description="Nothing waiting for review" />
						</template>
					</n-data-table>
					<div class="flex justify-end mt-12px">
						<bt-table-page v-bind="reviews.pageProps.value" @refresh="reviews.fetchTable" />
					</div>
				</n-tab-pane>
			</n-tabs>
		</n-drawer-content>
	</n-drawer>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NTag } from 'naive-ui'
import { useDataTable } from '@/hooks/useDataTable'
import { formatTime } from '@/utils/time'
import { isObject } from '@/utils'
import {
	getWorkflowLogs,
	getWorkflowReviews,
	getWorkflowRuns,
	resolveWorkflowReviews,
	type WorkflowReview,
	type Workflow,
	type WorkflowLog,
	type WorkflowRun,
} from '@/api/modules/workflow'
import { pathLabel, stepAtPath, stepLabel } from '../presets'

const show = defineModel<boolean>('show', { default: false })

const workflow = ref<Workflow | null>(null)
const tab = ref('runs')

const runs = useDataTable<WorkflowRun>({
	params: { workflow_id: 0, keyword: '', status: null as string | null, page: 1, page_size: 20 },
	immediate: false,
	fetchFn: getWorkflowRuns,
})

const logs = useDataTable<WorkflowLog>({
	params: { workflow_id: 0, status: null as string | null, page: 1, page_size: 20 },
	immediate: false,
	fetchFn: getWorkflowLogs,
})

const reviews = useDataTable<WorkflowReview>({
	params: { workflow_id: 0, status: 'pending' as string | null, page: 1, page_size: 20 },
	immediate: false,
	fetchFn: getWorkflowReviews,
})

// Selected rows come from the table hook (cleared on every reload)
const checked = reviews.tableKeys
const pendingCount = ref(0)

const loadPendingCount = async () => {
	if (!workflow.value) return
	const res = await getWorkflowReviews({ workflow_id: workflow.value.id, status: 'pending', page: 1, page_size: 1 })
	pendingCount.value = isObject<{ total: number }>(res) ? res.total : 0
}

const resolve = async (action: 'send' | 'discard') => {
	if (!workflow.value || !checked.value.length) return
	await resolveWorkflowReviews({ workflow_id: workflow.value.id, ids: checked.value.map(Number), action })
	reviews.fetchTable()
	logs.fetchTable()
	loadPendingCount()
}

const open = (w: Workflow) => {
	workflow.value = w
	tab.value = 'runs'
	runs.tableParams.value.workflow_id = w.id
	logs.tableParams.value.workflow_id = w.id
	reviews.tableParams.value.workflow_id = w.id
	reviews.tableParams.value.status = 'pending'
	runs.fetchTable(true)
	logs.fetchTable(true)
	reviews.fetchTable(true)
	loadPendingCount()
	show.value = true
}

defineExpose({ open })

const runStatusOptions = [
	{ label: 'In progress', value: 'active' },
	{ label: 'Finished', value: 'completed' },
	{ label: 'Stopped', value: 'exited' },
	{ label: "Didn't start (filters / re-entry)", value: 'skipped' },
]

const logStatusOptions = [
	{ label: 'Done', value: 'done' },
	{ label: 'Skipped', value: 'skipped' },
	{ label: 'Failed', value: 'failed' },
	{ label: 'Stopped', value: 'exited' },
	{ label: 'Held (quiet hours)', value: 'waiting' },
]

const runTag = {
	active: { type: 'info', text: 'In progress' },
	completed: { type: 'success', text: 'Finished' },
	exited: { type: 'warning', text: 'Stopped' },
	skipped: { type: 'default', text: "Didn't start" },
} as const

const logTag = {
	done: 'success',
	waiting: 'info',
	review: 'info',
	skipped: 'default',
	failed: 'error',
	exited: 'warning',
} as const

const stepText = (path: string, fallbackIndex: number) => {
	const steps = workflow.value?.steps ?? []
	const p = path || String(fallbackIndex)
	const step = stepAtPath(steps, p)
	return step ? `${pathLabel(steps, p)}. ${stepLabel(step.type)}` : pathLabel(steps, p)
}

const runColumns: DataTableColumns<WorkflowRun> = [
	{ title: 'Email', key: 'email', ellipsis: { tooltip: true } },
	{
		title: 'Status',
		key: 'status',
		width: 120,
		render: row => (
			<NTag type={runTag[row.status]?.type ?? 'default'} size="small">
				{runTag[row.status]?.text ?? row.status}
			</NTag>
		),
	},
	{
		title: 'Next step',
		key: 'step_index',
		width: 170,
		render: row => (row.status === 'active' ? stepText(row.step_path, row.step_index) : '-'),
	},
	{
		title: 'Runs at',
		key: 'next_run_at',
		width: 170,
		render: row => (row.status === 'active' ? formatTime(row.next_run_at) : '-'),
	},
	{ title: 'Started', key: 'create_time', width: 170, render: row => formatTime(row.create_time) },
	{ title: 'Last error', key: 'last_error', width: 180, ellipsis: { tooltip: true } },
]

const logColumns: DataTableColumns<WorkflowLog> = [
	{ title: 'Time', key: 'create_time', width: 170, render: row => formatTime(row.create_time) },
	{ title: 'Email', key: 'email', width: 200, ellipsis: { tooltip: true } },
	{
		title: 'Step',
		key: 'step_index',
		width: 190,
		render: row => `${pathLabel(workflow.value?.steps ?? [], row.step_path || String(row.step_index))}. ${stepLabel(row.step_type)}`,
	},
	{
		title: 'Result',
		key: 'status',
		width: 100,
		render: row => (
			<NTag type={logTag[row.status] ?? 'default'} size="small">
				{row.status}
			</NTag>
		),
	},
	{ title: 'Detail', key: 'detail', ellipsis: { tooltip: true } },
]

const reviewStatusOptions = [
	{ label: 'Waiting for review', value: 'pending' },
	{ label: 'Sent', value: 'sent' },
	{ label: 'Failed', value: 'failed' },
	{ label: 'Discarded', value: 'discarded' },
]

const reviewTag = {
	pending: 'info',
	sent: 'success',
	failed: 'error',
	discarded: 'default',
} as const

const reviewColumns: DataTableColumns<WorkflowReview> = [
	{ type: 'selection', disabled: row => row.status !== 'pending' },
	{ title: 'Queued', key: 'create_time', width: 170, render: row => formatTime(row.create_time) },
	{ title: 'Email', key: 'email', ellipsis: { tooltip: true } },
	{
		title: 'Message',
		key: 'step_path',
		width: 200,
		render: row => `${pathLabel(workflow.value?.steps ?? [], row.step_path)}. ${stepLabel(row.step_type)}`,
	},
	{
		title: 'Status',
		key: 'status',
		width: 110,
		render: row => (
			<NTag type={reviewTag[row.status] ?? 'default'} size="small">
				{row.status}
			</NTag>
		),
	},
	{ title: 'Result', key: 'detail', ellipsis: { tooltip: true } },
]
</script>
