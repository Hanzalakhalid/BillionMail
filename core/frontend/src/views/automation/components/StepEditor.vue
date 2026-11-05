<template>
	<div class="step-fields">
		<template v-if="step.type === 'wait'">
			<div class="flex items-center gap-8px">
				<n-input-number v-model:value="step.wait_value" :min="step.at_time ? 0 : 1" class="w-120px" />
				<n-select v-model:value="step.wait_unit" :options="waitUnitOptions" class="w-140px" @update:value="onUnitChange" />
			</div>
			<template v-if="step.wait_unit === 'days'">
				<div class="wait-extra">
					<n-checkbox :checked="!!step.at_time" @update:checked="toggleAtTime">Then send at a specific time</n-checkbox>
					<div v-if="step.at_time" class="flex items-center gap-8px mt-6px">
						<n-select v-model:value="step.at_time" :options="clockOptions" size="small" class="w-110px" filterable />
						<n-select
							v-model:value="step.timezone"
							:options="timezoneOptions"
							size="small"
							filterable
							tag
							class="w-170px" />
					</div>
				</div>
				<div class="wait-extra">
					<n-checkbox :checked="!!step.weekdays" @update:checked="toggleWeekdays">Only send on certain days</n-checkbox>
					<div v-if="step.weekdays" class="flex flex-wrap gap-6px mt-6px">
						<button
							v-for="d in weekdayOptions"
							:key="d.value"
							type="button"
							class="day"
							:class="{ on: step.weekdays.includes(d.value) }"
							@click="toggleDay(d.value)">
							{{ d.label }}
						</button>
					</div>
				</div>
				<p class="hint">
					With a time, days are counted on the calendar (e.g. "1 day, then 10:00" = tomorrow 10:00). A wait that lands on
					an unticked day moves to the next ticked day.
				</p>
			</template>
		</template>

		<template v-else-if="step.type === 'email'">
			<n-form-item label="Template" :show-feedback="false" class="mb-12px">
				<div class="flex w-full gap-8px">
					<n-select
						v-model:value="step.template_id"
						:options="templateOptions"
						placeholder="Choose an email template"
						filterable
						class="flex-1" />
					<n-button :disabled="!step.template_id" :loading="previewLoading" @click="openPreview">
						<i class="i-mdi-eye-outline mr-4px"></i>
						View
					</n-button>
				</div>
			</n-form-item>
			<n-form-item label="Subject" :show-feedback="false" class="mb-12px">
				<n-input v-model:value="step.subject" placeholder="Your check-up is due, {{ .Subscriber.name }}" />
			</n-form-item>
			<n-form-item label="From" :show-feedback="false">
				<div class="flex w-full">
					<from-select v-model:value="addresser" v-model:name="step.full_name" />
				</div>
			</n-form-item>
			<p class="hint">Opens and clicks are tracked, so later steps can check "opened the last email".</p>
		</template>

		<template v-else-if="step.type === 'sms'">
			<n-input
				v-model:value="step.message"
				type="textarea"
				:autosize="{ minRows: 2, maxRows: 6 }"
				maxlength="1600"
				show-count
				placeholder="Hi {name}, ..." />
			<p class="hint">{name}, {email} or any contact field is filled in for each contact.</p>
		</template>

		<template v-else-if="step.type === 'whatsapp'">
			<n-select
				v-if="waTemplates.length"
				:value="waKey"
				:options="waOptions"
				placeholder="Choose an approved template"
				filterable
				class="mb-8px"
				@update:value="selectWaTemplate" />
			<div v-else class="flex gap-8px mb-8px">
				<n-input v-model:value="step.wa_template" placeholder="Template name, e.g. review_request" />
				<n-input v-model:value="step.wa_language" placeholder="en" class="w-100px" />
			</div>
			<div v-for="(_, i) in step.wa_params ?? []" :key="i" class="flex items-center gap-8px mb-6px">
				<span class="w-48px text-desc">{{ paramLabel(i) }}</span>
				<n-input v-model:value="step.wa_params![i]" placeholder="{name}" />
			</div>
			<n-button v-if="!selectedWa" size="tiny" @click="(step.wa_params ??= []).push('')">Add value</n-button>
			<p class="hint">Only templates approved by Meta can be sent. Contacts need a phone field.</p>
		</template>

		<template v-else-if="step.type === 'notify'">
			<div class="flex flex-wrap gap-8px mb-8px">
				<n-radio-group v-model:value="step.notify_channel" size="small">
					<n-radio-button value="email">Email</n-radio-button>
					<n-radio-button value="sms">SMS</n-radio-button>
				</n-radio-group>
				<n-input
					v-model:value="step.notify_to"
					:placeholder="step.notify_channel === 'sms' ? 'Staff phones, e.g. 0501234567, 0559876543' : 'Staff emails, e.g. reception@clinic.ae'"
					class="flex-1 min-w-240px" />
			</div>
			<template v-if="step.notify_channel === 'email'">
				<n-input v-model:value="step.subject" placeholder="Subject (optional): New lead {name}" class="mb-8px" />
				<div class="flex w-full mb-8px">
					<from-select v-model:value="addresser" v-model:name="step.full_name" />
				</div>
			</template>
			<n-input
				v-model:value="step.message"
				type="textarea"
				:autosize="{ minRows: 2, maxRows: 5 }"
				placeholder="New lead: {name} ({email}, {phone}). Please call them today." />
			<p class="hint">Goes to your team, not to the contact. Contact details are added at the bottom.</p>
		</template>

		<template v-else-if="step.type === 'add_tag' || step.type === 'remove_tag'">
			<n-select v-model:value="step.tag_id" :options="tagOptions" placeholder="Choose a tag" filterable />
		</template>

		<template v-else-if="step.type === 'set_attribute'">
			<div class="flex flex-wrap gap-8px">
				<n-input v-model:value="step.attribute" placeholder="Field, e.g. last_contacted" class="w-200px" />
				<n-input v-model:value="step.value" placeholder="{today}" class="flex-1 min-w-200px" />
			</div>
			<p class="hint">{today}, {today+7} or {today-1} become a date. {name} and other fields work too.</p>
		</template>

		<template v-else-if="step.type === 'remove_attribute'">
			<n-input v-model:value="step.attribute" placeholder="Field to remove, e.g. coupon_code" class="w-260px" />
			<p class="hint">Deletes this field from the contact. Name and phone can't be removed.</p>
		</template>

		<template v-else-if="step.type === 'add_to_group'">
			<n-select v-model:value="step.group_id" :options="groupOptions" placeholder="Choose a group" filterable />
			<p class="hint">Copies the contact into this group. Workflows that start on that group will start for them.</p>
		</template>

		<template v-else-if="step.type === 'webhook'">
			<n-input v-model:value="step.webhook_url" placeholder="https://your-n8n.com/webhook/new-lead" class="mb-8px" />
			<n-input
				v-model:value="step.webhook_headers"
				type="textarea"
				:autosize="{ minRows: 1, maxRows: 4 }"
				placeholder="Headers (optional), one per line: Authorization: Bearer xyz" />
			<p class="hint">
				POSTs JSON with the workflow, the contact (email, subscribed) and all their fields. Any 2xx answer counts as done.
			</p>
		</template>

		<template v-else-if="step.type === 'condition'">
			<condition-fields :cond="step" :tag-options="tagOptions" :campaign-options="campaignOptions" />
			<p class="hint">Contacts who don't match leave the workflow here.</p>
		</template>

		<template v-else-if="step.type === 'split'">
			<n-radio-group v-model:value="step.split_mode" size="small" class="mb-8px">
				<n-radio-button value="condition">Yes / No</n-radio-button>
				<n-radio-button value="random">A/B test</n-radio-button>
			</n-radio-group>
			<template v-if="step.split_mode === 'random'">
				<div class="flex items-center gap-8px">
					<span>A gets</span>
					<n-input-number v-model:value="step.percent" :min="1" :max="99" class="w-110px" />
					<span>% of contacts, B gets {{ 100 - (step.percent ?? 50) }}%</span>
				</div>
				<p class="hint">Compare the two branches in the numbers on each step.</p>
			</template>
			<template v-else>
				<div class="flex items-center gap-8px flex-wrap">
					<span>If they</span>
					<condition-fields
						:cond="step"
						:tag-options="tagOptions"
						:campaign-options="campaignOptions" />
				</div>
			</template>
		</template>

		<!-- Email template preview -->
		<n-modal
			v-model:show="previewShow"
			preset="card"
			:title="previewTitle"
			:style="{ width: '860px', maxWidth: '95vw' }"
			:bordered="false">
			<div v-if="step.subject" class="preview-subject">
				<span class="preview-subject-label">Subject</span>
				{{ step.subject }}
			</div>
			<bt-preview v-if="previewHtml" :value="previewHtml" :height="600" />
			<n-empty v-else description="This template has no content yet" class="py-40px" />
			<template #footer>
				<div class="flex items-center justify-between">
					<span class="hint m-0">
						Placeholders like <code v-pre>{{ .Subscriber.name }}</code> are filled in for each contact when it sends.
					</span>
					<n-button @click="previewShow = false">Close</n-button>
				</div>
			</template>
		</n-modal>

		<!-- Klaviyo-style message settings -->
		<div v-if="isMessage" class="msg-settings">
			<div class="msg-row">
				<span class="msg-label">Status</span>
				<n-radio-group :value="step.status ?? 'live'" size="small" @update:value="(v: MessageStatus) => (step.status = v)">
					<n-radio-button value="live">Live</n-radio-button>
					<n-radio-button value="manual">Manual</n-radio-button>
					<n-radio-button value="draft">Draft</n-radio-button>
				</n-radio-group>
				<span class="hint m-0">{{ statusHint }}</span>
			</div>
			<div class="msg-row">
				<span class="msg-label">Smart sending</span>
				<n-switch
					size="small"
					:value="!step.skip_smart_sending"
					@update:value="(v: boolean) => (step.skip_smart_sending = !v)" />
				<span class="hint m-0">Uses the workflow's smart sending window when it is on.</span>
			</div>
			<div class="msg-row items-start">
				<span class="msg-label mt-4px">Only send if</span>
				<div class="flex-1">
					<div v-for="(f, i) in step.filters ?? []" :key="i" class="flex items-center gap-6px mb-6px">
						<span class="text-12px whitespace-nowrap">they</span>
						<condition-fields :cond="f" :tag-options="tagOptions" :campaign-options="campaignOptions" />
						<n-button quaternary size="tiny" type="error" @click="step.filters!.splice(i, 1)">
							<i class="i-mdi-close"></i>
						</n-button>
					</div>
					<n-button size="tiny" @click="(step.filters ??= []).push({ condition: 'has_tag' })">
						<i class="i-mdi-plus mr-2px"></i>
						Add filter
					</n-button>
					<span class="hint ml-8px">People who don't match skip this message but stay in the workflow.</span>
				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts" setup>
import type { SelectOption } from 'naive-ui'
import type { MessageStatus, WorkflowStep } from '@/api/modules/workflow'
import type { WhatsappTemplate } from '@/api/modules/whatsapp'
import { getTemplateDetails } from '@/api/modules/market/template'
import { isObject } from '@/utils'
import FromSelect from '@/views/market/task/components/FromSelect.vue'
import ConditionFields from './ConditionFields.vue'
import { isMessageStep, weekdayOptions } from '../presets'

const step = defineModel<WorkflowStep>('step', { required: true })

const props = defineProps<{
	templateOptions: SelectOption[]
	tagOptions: SelectOption[]
	campaignOptions: SelectOption[]
	groupOptions: SelectOption[]
	waTemplates: WhatsappTemplate[]
}>()

const waitUnitOptions = [
	{ label: 'Minutes', value: 'minutes' },
	{ label: 'Hours', value: 'hours' },
	{ label: 'Days', value: 'days' },
]

// Wait: time of day and weekdays only make sense with days
const clockOptions = Array.from({ length: 48 }, (_, i) => {
	const v = `${String(Math.floor(i / 2)).padStart(2, '0')}:${i % 2 ? '30' : '00'}`
	return { label: v, value: v }
})

const timezoneOptions = ['Asia/Dubai', 'Asia/Riyadh', 'Asia/Qatar', 'Asia/Karachi', 'Europe/London', 'UTC'].map(z => ({
	label: z,
	value: z,
}))

const onUnitChange = (unit: string) => {
	if (unit !== 'days') {
		step.value.at_time = undefined
		step.value.weekdays = undefined
		if (!step.value.wait_value) step.value.wait_value = 1
	}
}

const toggleAtTime = (on: boolean) => {
	step.value.at_time = on ? '10:00' : undefined
	if (on) step.value.timezone ??= 'Asia/Dubai'
	if (!on && !step.value.wait_value) step.value.wait_value = 1
}

const toggleWeekdays = (on: boolean) => {
	step.value.weekdays = on ? [1, 2, 3, 4, 5] : undefined
	if (on) step.value.timezone ??= 'Asia/Dubai'
}

const toggleDay = (d: number) => {
	const days = step.value.weekdays ?? []
	step.value.weekdays = days.includes(d) ? days.filter(x => x !== d) : [...days, d].sort()
}

// Email template preview
const previewShow = ref(false)
const previewLoading = ref(false)
const previewHtml = ref('')
const previewTitle = ref('Template preview')

const openPreview = async () => {
	const id = step.value.template_id
	if (!id) return
	previewLoading.value = true
	try {
		const res = await getTemplateDetails({ id: `${id}` })
		if (!isObject<{ temp_name: string; html_content: string }>(res)) return
		previewHtml.value = res.html_content || ''
		previewTitle.value = `Template: ${res.temp_name}`
		previewShow.value = true
	} finally {
		previewLoading.value = false
	}
}

// Message settings
const isMessage = computed(() => isMessageStep(step.value.type))

const statusHint = computed(
	() =>
		({
			live: 'Sends automatically.',
			manual: 'Waits in "Needs review" until you send it.',
			draft: 'Not sent; contacts skip it.',
		})[step.value.status ?? 'live']
)

// FromSelect picks the first mailbox only when the value is null
const addresser = computed<string | null>({
	get: () => step.value.addresser || null,
	set: v => {
		step.value.addresser = v ?? ''
	},
})

const paramLabel = (i: number) => `{{${i + 1}}}`

const waKeyOf = (name: string, language: string) => `${name}|${language}`

const waOptions = computed<SelectOption[]>(() =>
	props.waTemplates.map(t => ({ label: `${t.name} (${t.language})`, value: waKeyOf(t.name, t.language) }))
)

const waKey = computed(() =>
	step.value.wa_template ? waKeyOf(step.value.wa_template, step.value.wa_language ?? '') : null
)

const selectedWa = computed(() => props.waTemplates.find(t => waKeyOf(t.name, t.language) === waKey.value))

const selectWaTemplate = (key: string) => {
	const t = props.waTemplates.find(item => waKeyOf(item.name, item.language) === key)
	if (!t) return
	const count = Math.max(0, ...[...t.body.matchAll(/\{\{(\d+)\}\}/g)].map(m => Number(m[1])))
	step.value.wa_template = t.name
	step.value.wa_language = t.language
	step.value.wa_params = Array.from({ length: count }, (_, i) => step.value.wa_params?.[i] ?? '')
}
</script>

<style lang="scss" scoped>
.preview-subject {
	padding: 8px 12px;
	margin-bottom: 10px;
	font-size: 13px;
	background: var(--color-fill-1, #f7f8fa);
	border-radius: 6px;
}

.preview-subject-label {
	margin-right: 8px;
	font-weight: 600;
	color: var(--color-text-3, #888);
}

.wait-extra {
	margin-top: 10px;
}

.day {
	min-width: 44px;
	padding: 3px 8px;
	font-size: 12px;
	color: inherit;
	cursor: pointer;
	background: transparent;
	border: 1px solid var(--color-border-3, #c9cdd4);
	border-radius: 12px;

	&.on {
		color: #fff;
		background: var(--color-primary-1, #18a058);
		border-color: var(--color-primary-1, #18a058);
	}
}

.msg-settings {
	padding: 10px 12px;
	margin-top: 12px;
	background: var(--color-fill-1, #f7f8fa);
	border-radius: 8px;
}

.msg-row {
	display: flex;
	flex-wrap: wrap;
	gap: 10px;
	align-items: center;

	& + .msg-row {
		margin-top: 8px;
	}
}

.msg-label {
	width: 96px;
	font-size: 12px;
	font-weight: 600;
}

.hint {
	margin: 6px 0 0;
	font-size: 12px;
	color: var(--color-text-3, #888);
}
</style>
