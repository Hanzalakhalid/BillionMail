<template>
	<div>
		<div class="field-label">What starts this flow?</div>
		<div class="trigger-types">
			<button
				v-for="t in triggerTypes"
				:key="t.value"
				type="button"
				class="trigger-type"
				:class="{ active: trigger.type === t.value }"
				@click="trigger.type = t.value">
				<i :class="t.icon"></i>
				<span>{{ t.label }}</span>
			</button>
		</div>

		<template v-if="trigger.type === 'group_join'">
			<div class="field-label">Group</div>
			<n-select v-model:value="group" :options="groupOptions" placeholder="Choose a contact group" filterable />
			<p class="hint">Starts when a new contact is added to this group (import, form or API).</p>
		</template>

		<template v-else-if="trigger.type === 'tag_added'">
			<div class="field-label">Tag</div>
			<n-select v-model:value="tag" :options="tagOptions" placeholder="Choose a tag, e.g. Visited" filterable />
			<p class="hint">Starts each time this tag is added to a contact.</p>
		</template>

		<template v-else-if="trigger.type === 'email_opened' || trigger.type === 'email_clicked'">
			<div class="field-label">Campaign</div>
			<n-select v-model:value="task" :options="campaignOptions" placeholder="Choose an email campaign" filterable />
			<p class="hint">
				Starts when a contact {{ trigger.type === 'email_opened' ? 'opens' : 'clicks a link in' }} this campaign.
				Needs open/click tracking on the campaign.
			</p>
		</template>

		<template v-else>
			<div class="field-label">When</div>
			<div class="flex flex-wrap items-center gap-8px">
				<n-input-number v-model:value="offsetAbs" :min="0" :max="3650" class="w-100px" />
				<span>days</span>
				<n-select v-model:value="offsetDir" :options="offsetDirOptions" class="w-100px" />
			</div>
			<div class="field-label">Date field</div>
			<n-input v-model:value="trigger.attribute" placeholder="last_visit" />
			<div class="field-label">Repeat</div>
			<n-radio-group :value="recurrence" size="small" @update:value="setRecurrence">
				<n-radio-button value="once">Once per date</n-radio-button>
				<n-radio-button value="monthly">Every month</n-radio-button>
				<n-radio-button value="yearly">Every year</n-radio-button>
			</n-radio-group>
			<div class="field-label">Contacts in</div>
			<n-select v-model:value="group" :options="[{ label: 'All groups', value: 0 }, ...groupOptions]" />
			<p class="hint">Uses a contact field holding a date like 2026-05-08 or 08/05/2026. Checked every 10 minutes.</p>
		</template>

		<n-checkbox v-if="trigger.type !== 'date' && !isEdit" v-model:checked="trigger.include_existing" class="mt-12px">
			Also start for contacts who already match
		</n-checkbox>

		<div class="divider"></div>
		<div class="field-label mt-0">Re-entry</div>
		<n-select v-model:value="trigger.reentry" :options="reentryOptions" />
		<div v-if="trigger.reentry === 'after'" class="flex items-center gap-8px mt-8px">
			<span>After</span>
			<n-input-number v-model:value="trigger.reentry_days" :min="1" :max="3650" class="w-100px" />
			<span>days</span>
		</div>
		<p class="hint">Can someone go through this flow more than once?</p>
	</div>
</template>

<script lang="ts" setup>
import type { SelectOption } from 'naive-ui'
import type { TriggerType, WorkflowTrigger } from '@/api/modules/workflow'

const trigger = defineModel<WorkflowTrigger>('trigger', { required: true })

defineProps<{
	isEdit: boolean
	groupOptions: SelectOption[]
	tagOptions: SelectOption[]
	campaignOptions: SelectOption[]
}>()

const triggerTypes: { label: string; value: TriggerType; icon: string }[] = [
	{ label: 'Joins a group', value: 'group_join', icon: 'i-mdi-account-multiple-plus-outline' },
	{ label: 'Gets a tag', value: 'tag_added', icon: 'i-mdi-tag-outline' },
	{ label: 'Date', value: 'date', icon: 'i-mdi-calendar-clock' },
	{ label: 'Opens email', value: 'email_opened', icon: 'i-mdi-email-open-outline' },
	{ label: 'Clicks email', value: 'email_clicked', icon: 'i-mdi-cursor-default-click-outline' },
]

// Selects use null for "nothing chosen" so the placeholder shows
const group = computed<number | null>({
	get: () => (trigger.value.type === 'date' ? trigger.value.group_id : trigger.value.group_id || null),
	set: v => {
		trigger.value.group_id = v ?? 0
	},
})

const tag = computed<number | null>({
	get: () => trigger.value.tag_id || null,
	set: v => {
		trigger.value.tag_id = v ?? 0
	},
})

const task = computed<number | null>({
	get: () => trigger.value.task_id || null,
	set: v => {
		trigger.value.task_id = v ?? 0
	},
})

const offsetDirOptions = [
	{ label: 'after', value: 1 },
	{ label: 'before', value: -1 },
]

const offsetAbs = computed<number>({
	get: () => Math.abs(trigger.value.offset_days),
	set: v => {
		const dir = trigger.value.offset_days < 0 ? -1 : 1
		trigger.value.offset_days = dir * (v ?? 0)
	},
})

const offsetDir = computed<number>({
	get: () => (trigger.value.offset_days < 0 ? -1 : 1),
	set: dir => {
		trigger.value.offset_days = dir * Math.abs(trigger.value.offset_days)
	},
})

const recurrence = computed(() => (trigger.value.yearly ? 'yearly' : trigger.value.monthly ? 'monthly' : 'once'))

const setRecurrence = (v: string) => {
	trigger.value.yearly = v === 'yearly'
	trigger.value.monthly = v === 'monthly'
}

const reentryOptions = [
	{ label: 'Every time they qualify', value: 'always' },
	{ label: 'Only once, ever', value: 'once' },
	{ label: 'Again after some days', value: 'after' },
]
</script>

<style lang="scss" scoped>
.field-label {
	margin: 14px 0 6px;
	font-size: 13px;
	font-weight: 600;
}

.mt-0 {
	margin-top: 0;
}

.trigger-types {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 8px;
}

.trigger-type {
	display: flex;
	flex-direction: column;
	gap: 4px;
	align-items: center;
	padding: 10px 4px;
	font-size: 12px;
	color: inherit;
	cursor: pointer;
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 8px;

	i {
		font-size: 20px;
	}

	&:hover {
		border-color: var(--color-primary-1, #18a058);
	}

	&.active {
		color: var(--color-primary-1, #18a058);
		background: rgb(24 160 88 / 8%);
		border-color: var(--color-primary-1, #18a058);
	}
}

.divider {
	margin: 18px 0 14px;
	border-top: 1px dashed var(--color-border-2, #e5e6eb);
}

.hint {
	margin: 6px 0 0;
	font-size: 12px;
	line-height: 1.5;
	color: var(--color-text-3, #888);
}
</style>
