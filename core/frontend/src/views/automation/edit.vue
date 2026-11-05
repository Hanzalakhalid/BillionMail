<template>
	<div class="builder">
		<!-- Top bar -->
		<header class="topbar">
			<n-button quaternary circle title="Back to workflows" @click="goBack">
				<i class="i-mdi-arrow-left text-20px"></i>
			</n-button>
			<div class="title-wrap">
				<input v-model="form.name" class="title-input" placeholder="Name this flow, e.g. Review request after visit" />
				<div class="title-meta">
					<n-tag :type="statusTag.type" size="small" round>{{ statusTag.text }}</n-tag>
					<span class="text-desc">{{ countSteps(form.steps) }} steps</span>
				</div>
			</div>
			<n-button quaternary @click="select(SETTINGS)">
				<i class="i-mdi-cog-outline mr-4px"></i>
				Flow settings
			</n-button>
			<span v-if="problem" class="problem" :title="problem">
				<i class="i-mdi-alert-circle-outline"></i>
				{{ problem }}
			</span>
			<span v-else class="ready">
				<i class="i-mdi-check-circle-outline"></i>
				Ready to save
			</span>
			<n-button @click="goBack">Cancel</n-button>
			<n-button type="primary" :disabled="!!problem" :loading="saving" @click="save">Save</n-button>
		</header>

		<n-spin :show="loading" class="flex-1 min-h-0">
			<div class="body">
				<!-- Left: actions palette -->
				<aside class="left">
					<action-palette :hint="addHint" @add="addFromPalette" />
				</aside>

				<!-- Middle: the flow -->
				<main class="canvas" @click="select(SETTINGS)">
					<n-alert v-if="status === 'active'" type="warning" class="canvas-alert" @click.stop>
						This flow is on. Changes apply to contacts already inside it. Pause it first for big changes to branches.
					</n-alert>

					<div class="flow">
						<div class="trigger-node" :class="{ selected: selected === TRIGGER }" @click.stop="select(TRIGGER)">
							<div class="trigger-icon"><i class="i-mdi-lightning-bolt"></i></div>
							<div class="min-w-0 flex-1">
								<div class="trigger-kicker">Trigger</div>
								<div class="trigger-text">{{ triggerText }}</div>
								<div v-if="triggerProblem" class="trigger-problem">
									<i class="i-mdi-alert-circle-outline"></i>
									{{ triggerProblem }}
								</div>
							</div>
						</div>

						<flow-canvas :steps="form.steps" list-path="" :depth="0" />

						<div class="end-node">
							<i class="i-mdi-flag-checkered"></i>
							End
						</div>
					</div>
				</main>

				<!-- Right: inspector for whatever is selected -->
				<aside class="right">
					<template v-if="selected === TRIGGER">
						<div class="panel-head">
							<span class="panel-icon trigger"><i class="i-mdi-lightning-bolt"></i></span>
							<div class="panel-title">Trigger</div>
						</div>
						<trigger-panel
							v-model:trigger="form.trigger"
							:is-edit="!!editId"
							:group-options="groupOptions"
							:tag-options="tagOptions"
							:campaign-options="campaignOptions" />
					</template>

					<template v-else-if="selectedStep">
						<div class="panel-head">
							<span class="panel-icon" :class="`cat-${stepCategory(selectedStep.type)}`">
								<i :class="stepIcon(selectedStep.type)"></i>
							</span>
							<div class="flex-1 min-w-0">
								<div class="panel-kicker">Step {{ pathLabel(form.steps, selected) }}</div>
								<n-select
									:value="selectedStep.type"
									:options="typeOptions"
									size="small"
									class="w-220px"
									@update:value="changeType" />
							</div>
							<n-button quaternary circle size="small" title="Close" @click="select(SETTINGS)">
								<i class="i-mdi-close"></i>
							</n-button>
						</div>
						<step-editor
							:key="selected"
							:step="selectedStep"
							:template-options="templateOptions"
							:tag-options="tagOptions"
							:campaign-options="campaignOptions"
							:group-options="groupOptions"
							:wa-templates="waTemplates" />
						<p v-if="selectedStep.type === 'split'" class="hint mt-12px">
							Add steps under the {{ selectedStep.split_mode === 'random' ? 'A and B' : 'YES and NO' }} branches in the
							flow.
						</p>
						<div class="panel-actions">
							<n-button size="small" @click="act(selected, 'duplicate')">
								<i class="i-mdi-content-copy mr-4px"></i>
								Duplicate
							</n-button>
							<n-button size="small" type="error" ghost @click="act(selected, 'delete')">
								<i class="i-mdi-delete-outline mr-4px"></i>
								Delete
							</n-button>
						</div>
					</template>

					<template v-else>
						<div class="panel-head">
							<span class="panel-icon settings"><i class="i-mdi-cog-outline"></i></span>
							<div class="panel-title">Flow settings</div>
						</div>
						<p class="hint mt-0">Click any box in the flow to edit it.</p>
						<settings-panel
							v-model:settings="form.settings"
							:tag-options="tagOptions"
							:campaign-options="campaignOptions" />
					</template>
				</aside>
			</div>
		</n-spin>
	</div>
</template>

<script lang="ts" setup>
import {
	createWorkflow,
	getWorkflowDetail,
	getWorkflowStepStats,
	updateWorkflow,
	type StepType,
	type Workflow,
	type WorkflowPayload,
	type WorkflowStepStats,
} from '@/api/modules/workflow'
import { isObject } from '@/utils'
import {
	clonePayload,
	countSteps,
	describeTrigger,
	duplicateStep,
	emptyPayload,
	emptySettings,
	emptyTrigger,
	insertStep,
	listAt,
	moveStep,
	newStep,
	normalizeSteps,
	parentOf,
	pathLabel,
	presets,
	removeStep,
	stepAtPath,
	stepCategory,
	stepIcon,
	stepTypeOptions,
	validatePayload,
} from './presets'
import { builderKey, SETTINGS, TRIGGER, type NodeAction } from './builder'
import { useWorkflowLookups } from './useWorkflowLookups'
import FlowCanvas from './components/FlowCanvas.vue'
import ActionPalette from './components/ActionPalette.vue'
import StepEditor from './components/StepEditor.vue'
import TriggerPanel from './components/TriggerPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'

const route = useRoute()
const router = useRouter()

const { groupOptions, tagOptions, templateOptions, campaignOptions, waTemplates, lookups, load: loadLookups } =
	useWorkflowLookups()

const editId = ref(0)
const status = ref<Workflow['status']>('draft')
const form = ref<WorkflowPayload>(emptyPayload())
const stats = ref<Record<string, WorkflowStepStats>>({})
const loading = ref(false)
const saving = ref(false)

// ---------- Selection and editing (shared with the canvas through provide) ----------
const selected = ref<string>(TRIGGER)
const dragType = ref<StepType | null>(null)

const select = (path: string) => {
	selected.value = path
}

const selectedStep = computed(() =>
	selected.value === TRIGGER || selected.value === SETTINGS ? undefined : stepAtPath(form.value.steps, selected.value)
)

const add = (listPath: string, index: number, type: StepType) => {
	const path = insertStep(form.value.steps, listPath, index, type)
	if (path) select(path)
}

/** Clicking a palette item adds the step right after the selected step, or at the end. */
const addFromPalette = (type: StepType) => {
	if (selectedStep.value) {
		const { listPath, index } = parentOf(selected.value)
		add(listPath, index + 1, type)
	} else {
		add('', form.value.steps.length, type)
	}
}

const addHint = computed(() => (selectedStep.value ? `after step ${pathLabel(form.value.steps, selected.value)}` : 'at the end'))

const act = (path: string, action: NodeAction) => {
	const steps = form.value.steps
	if (action === 'delete') {
		removeStep(steps, path)
		select(SETTINGS)
	} else if (action === 'duplicate') {
		select(duplicateStep(steps, path) || path)
	} else {
		select(moveStep(steps, path, action === 'up' ? -1 : 1))
	}
}

const typeOptions = computed(() => stepTypeOptions.map(o => ({ label: o.label, value: o.value })))

const changeType = (type: StepType) => {
	const { listPath, index } = parentOf(selected.value)
	const list = listAt(form.value.steps, listPath)
	if (list) list[index] = newStep(type)
}

provide(builderKey, { selected, select, stats, lookups, dragType, add, act })

// ---------- Loading ----------
/** Fills in defaults for anything an older or preset workflow leaves out. */
const prepare = (payload: WorkflowPayload, existing: boolean): WorkflowPayload => {
	const copy = clonePayload(payload)
	copy.trigger = { ...emptyTrigger(), ...copy.trigger }
	if (!copy.trigger.reentry) copy.trigger.reentry = 'always'
	if (!copy.trigger.reentry_days) copy.trigger.reentry_days = 30
	const defaults = emptySettings()
	copy.settings = {
		...defaults,
		...(copy.settings ?? {}),
		quiet_hours: { ...defaults.quiet_hours, ...(copy.settings?.quiet_hours ?? {}) },
		exit_conditions: copy.settings?.exit_conditions ?? [],
		flow_filters: copy.settings?.flow_filters ?? [],
	}
	// Existing workflows saved without quiet hours keep them off
	if (existing && !payload.settings?.quiet_hours?.timezone) copy.settings.quiet_hours.enabled = false
	copy.steps = normalizeSteps(copy.steps ?? [])
	return copy
}

const loadWorkflow = async () => {
	const id = Number(route.query.id || 0)
	if (!id) {
		const preset = presets.find(p => p.key === route.query.preset)
		form.value = prepare(preset ? preset.payload : emptyPayload(), false)
		select(TRIGGER)
		return
	}
	loading.value = true
	try {
		const res = await getWorkflowDetail(id)
		if (!isObject<Workflow>(res)) return
		editId.value = res.id
		status.value = res.status
		form.value = prepare({ name: res.name, trigger: res.trigger, steps: res.steps, settings: res.settings }, true)
		select(SETTINGS)
		const s = await getWorkflowStepStats(id)
		if (isObject<Record<string, WorkflowStepStats>>(s)) stats.value = s
	} finally {
		loading.value = false
	}
}

// ---------- Header ----------
const statusTag = computed(() => {
	if (!editId.value) return { type: 'default' as const, text: 'New' }
	return {
		draft: { type: 'default' as const, text: 'Draft' },
		active: { type: 'success' as const, text: 'On' },
		paused: { type: 'warning' as const, text: 'Paused' },
	}[status.value]
})

const triggerText = computed(() => describeTrigger(form.value.trigger, lookups))

const problem = computed(() => validatePayload(form.value))

// Trigger problems are shown on the trigger node itself
const triggerProblem = computed(() => {
	const p = problem.value
	return /^(Choose the group|Choose the tag|Choose the email campaign|Enter the attribute|Re-entry)/.test(p) ? p : ''
})

const goBack = () => {
	router.push('/automation')
}

const save = async () => {
	if (problem.value) return
	saving.value = true
	try {
		if (editId.value) {
			await updateWorkflow({ ...form.value, id: editId.value })
		} else {
			await createWorkflow(form.value)
		}
		goBack()
	} finally {
		saving.value = false
	}
}

onMounted(() => {
	loadLookups()
	loadWorkflow()
})
</script>

<style lang="scss" scoped>
.builder {
	display: flex;
	flex-direction: column;
	height: calc(100vh - 64px);
	min-height: 600px;
}

.topbar {
	display: flex;
	flex: none;
	gap: 10px;
	align-items: center;
	padding: 10px 20px;
	background: var(--color-bg-2, #fff);
	border-bottom: 1px solid var(--color-border-2, #e5e6eb);
}

.title-wrap {
	flex: 1;
	min-width: 0;
}

.title-input {
	width: 100%;
	padding: 2px 0;
	font-size: 18px;
	font-weight: 600;
	color: inherit;
	background: transparent;
	border: none;
	border-bottom: 1px dashed transparent;
	outline: none;

	&:hover,
	&:focus {
		border-bottom-color: var(--color-border-3, #c9cdd4);
	}

	&::placeholder {
		font-weight: 400;
		color: var(--color-text-4, #aaa);
	}
}

.title-meta {
	display: flex;
	gap: 10px;
	align-items: center;
	font-size: 12px;
}

.problem,
.ready {
	display: inline-flex;
	gap: 4px;
	align-items: center;
	max-width: 320px;
	overflow: hidden;
	font-size: 13px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.problem {
	color: #d03050;
}

.ready {
	color: var(--color-primary-1, #18a058);
}

.body {
	display: grid;
	grid-template-columns: 230px minmax(0, 1fr) 400px;
	height: 100%;
	min-height: 0;
}

.left,
.right {
	min-height: 0;
	overflow-y: auto;
	background: var(--color-bg-2, #fff);
}

.left {
	border-right: 1px solid var(--color-border-2, #e5e6eb);
}

.right {
	padding: 16px 18px 40px;
	border-left: 1px solid var(--color-border-2, #e5e6eb);
}

.canvas {
	min-height: 0;
	padding: 28px 24px 80px;
	overflow: auto;
	background-color: var(--color-fill-1, #f5f6f8);
	background-image: radial-gradient(var(--color-border-2, #d9dce3) 1px, transparent 1px);
	background-size: 18px 18px;
}

.canvas-alert {
	max-width: 560px;
	margin: 0 auto 16px;
}

.flow {
	display: flex;
	flex-direction: column;
	align-items: center;
	width: max-content;
	min-width: 100%;
}

.trigger-node {
	display: flex;
	gap: 12px;
	align-items: center;
	width: 340px;
	padding: 14px;
	cursor: pointer;
	background: var(--color-bg-2, #fff);
	border: 2px solid var(--color-primary-1, #18a058);
	border-radius: 12px;
	box-shadow: 0 2px 8px rgb(0 0 0 / 6%);

	&.selected {
		box-shadow:
			0 0 0 4px rgb(24 160 88 / 20%),
			0 4px 14px rgb(0 0 0 / 10%);
	}
}

.trigger-icon {
	display: inline-flex;
	flex: none;
	align-items: center;
	justify-content: center;
	width: 36px;
	height: 36px;
	font-size: 20px;
	color: #fff;
	background: var(--color-primary-1, #18a058);
	border-radius: 10px;
}

.trigger-kicker {
	font-size: 11px;
	font-weight: 700;
	color: var(--color-primary-1, #18a058);
	text-transform: uppercase;
	letter-spacing: 0.5px;
}

.trigger-text {
	font-size: 14px;
	font-weight: 600;
}

.trigger-problem {
	display: flex;
	gap: 4px;
	align-items: center;
	margin-top: 2px;
	font-size: 12px;
	color: #d03050;
}

.end-node {
	display: flex;
	gap: 6px;
	align-items: center;
	padding: 6px 16px;
	font-size: 13px;
	font-weight: 600;
	color: var(--color-text-3, #888);
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 16px;
}

.panel-head {
	display: flex;
	gap: 12px;
	align-items: center;
	padding-bottom: 14px;
	margin-bottom: 14px;
	border-bottom: 1px solid var(--color-border-2, #e5e6eb);
}

.panel-icon {
	--accent: #2080f0;

	display: inline-flex;
	flex: none;
	align-items: center;
	justify-content: center;
	width: 34px;
	height: 34px;
	font-size: 18px;
	color: var(--accent);
	background: color-mix(in srgb, var(--accent) 12%, transparent);
	border-radius: 8px;

	&.trigger {
		--accent: #18a058;
	}

	&.settings {
		--accent: #666;
	}

	&.cat-timing {
		--accent: #f0a020;
	}

	&.cat-logic {
		--accent: #8a63d2;
	}

	&.cat-data {
		--accent: #18a058;
	}
}

.panel-title {
	font-size: 16px;
	font-weight: 600;
}

.panel-kicker {
	margin-bottom: 4px;
	font-size: 11px;
	font-weight: 700;
	color: var(--color-text-3, #888);
	text-transform: uppercase;
}

.panel-actions {
	display: flex;
	gap: 8px;
	padding-top: 14px;
	margin-top: 18px;
	border-top: 1px solid var(--color-border-2, #e5e6eb);
}

.text-desc {
	color: var(--color-text-3, #888);
}

.hint {
	margin: 6px 0 0;
	font-size: 12px;
	line-height: 1.5;
	color: var(--color-text-3, #888);
}

@media (width <= 1280px) {
	.body {
		grid-template-columns: 200px minmax(0, 1fr) 360px;
	}
}
</style>
