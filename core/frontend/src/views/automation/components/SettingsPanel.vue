<template>
	<div>
		<section class="block">
			<div class="block-head">
				<div>
					<div class="block-title"><i class="i-mdi-weather-night"></i> Quiet hours</div>
					<p class="hint">Messages due at night wait until morning. Staff alerts are never held.</p>
				</div>
				<n-switch v-model:value="settings.quiet_hours.enabled" />
			</div>
			<div v-if="settings.quiet_hours.enabled" class="block-body">
				<div class="flex items-center gap-8px">
					<n-select v-model:value="settings.quiet_hours.start" :options="hourOptions" size="small" />
					<span>to</span>
					<n-select v-model:value="settings.quiet_hours.end" :options="hourOptions" size="small" />
				</div>
				<n-select
					v-model:value="settings.quiet_hours.timezone"
					:options="timezoneOptions"
					filterable
					tag
					size="small"
					class="mt-8px" />
			</div>
		</section>

		<section class="block">
			<div class="block-head">
				<div>
					<div class="block-title"><i class="i-mdi-shield-check-outline"></i> Smart sending</div>
					<p class="hint">Skip a message if the contact got one from any workflow recently.</p>
				</div>
				<n-switch :value="settings.smart_sending_hours > 0" @update:value="toggleSmart" />
			</div>
			<div v-if="settings.smart_sending_hours > 0" class="block-body flex items-center gap-8px">
				<span>Within the last</span>
				<n-input-number v-model:value="settings.smart_sending_hours" :min="1" :max="720" size="small" class="w-90px" />
				<span>hours</span>
			</div>
		</section>

		<section class="block">
			<div class="block-title"><i class="i-mdi-filter-outline"></i> Flow filters</div>
			<p class="hint">
				All must be true. Checked when someone enters (they don't start if not) and before every message (that message
				is skipped, they stay in).
			</p>
			<div v-for="(c, i) in settings.flow_filters" :key="i" class="cond-row">
				<div class="cond-head">
					<span>Only if they</span>
					<n-button quaternary size="tiny" type="error" @click="settings.flow_filters.splice(i, 1)">
						<i class="i-mdi-close"></i>
					</n-button>
				</div>
				<condition-fields :cond="c" :tag-options="tagOptions" :campaign-options="campaignOptions" no-last-email />
			</div>
			<n-button size="small" dashed block class="mt-8px" @click="settings.flow_filters.push({ condition: 'subscribed' })">
				<i class="i-mdi-plus mr-4px"></i>
				Add flow filter
			</n-button>
		</section>

		<section class="block">
			<div class="block-title"><i class="i-mdi-flag-checkered"></i> Exit when the goal is reached</div>
			<p class="hint">Checked before every step. If any is true, the contact leaves (e.g. they booked).</p>
			<div v-for="(c, i) in settings.exit_conditions" :key="i" class="cond-row">
				<div class="cond-head">
					<span>Exit if they</span>
					<n-button quaternary size="tiny" type="error" @click="settings.exit_conditions.splice(i, 1)">
						<i class="i-mdi-close"></i>
					</n-button>
				</div>
				<condition-fields :cond="c" :tag-options="tagOptions" :campaign-options="campaignOptions" />
			</div>
			<n-button size="small" dashed block class="mt-8px" @click="settings.exit_conditions.push({ condition: 'has_tag' })">
				<i class="i-mdi-plus mr-4px"></i>
				Add exit condition
			</n-button>
		</section>
	</div>
</template>

<script lang="ts" setup>
import type { SelectOption } from 'naive-ui'
import type { WorkflowSettings } from '@/api/modules/workflow'
import ConditionFields from './ConditionFields.vue'

const settings = defineModel<WorkflowSettings>('settings', { required: true })

defineProps<{
	tagOptions: SelectOption[]
	campaignOptions: SelectOption[]
}>()

const hourOptions = Array.from({ length: 24 }, (_, h) => ({ label: `${String(h).padStart(2, '0')}:00`, value: h }))

const timezoneOptions = ['Asia/Dubai', 'Asia/Riyadh', 'Asia/Qatar', 'Asia/Karachi', 'Europe/London', 'UTC'].map(z => ({
	label: z,
	value: z,
}))

const toggleSmart = (on: boolean) => {
	settings.value.smart_sending_hours = on ? 16 : 0
}
</script>

<style lang="scss" scoped>
.block {
	padding: 14px 0;

	& + .block {
		border-top: 1px solid var(--color-border-2, #e5e6eb);
	}
}

.block-head {
	display: flex;
	gap: 12px;
	align-items: flex-start;
	justify-content: space-between;
}

.block-title {
	display: flex;
	gap: 6px;
	align-items: center;
	font-weight: 600;
}

.block-body {
	padding-top: 10px;
}

.cond-row {
	padding: 10px;
	margin-top: 8px;
	background: var(--color-fill-1, #f7f8fa);
	border-radius: 8px;
}

.cond-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 6px;
	font-size: 13px;
}

.hint {
	margin: 4px 0 0;
	font-size: 12px;
	line-height: 1.5;
	color: var(--color-text-3, #888);
}
</style>
