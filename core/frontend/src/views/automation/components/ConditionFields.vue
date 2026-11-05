<template>
	<div class="flex flex-wrap gap-8px">
		<n-select v-model:value="cond.condition" :options="options" class="w-220px" />
		<n-select
			v-if="cond.condition === 'has_tag' || cond.condition === 'not_has_tag'"
			v-model:value="cond.tag_id"
			:options="tagOptions"
			placeholder="Choose a tag"
			filterable
			class="flex-1 min-w-200px" />
		<n-select
			v-if="cond.condition === 'opened_campaign' || cond.condition === 'clicked_campaign'"
			v-model:value="cond.task_id"
			:options="campaignOptions"
			placeholder="Choose a campaign"
			filterable
			class="flex-1 min-w-200px" />
		<template v-if="cond.condition === 'attr_equals' || cond.condition === 'attr_set'">
			<n-input v-model:value="cond.attribute" placeholder="Field, e.g. treatment" class="w-180px" />
			<n-input
				v-if="cond.condition === 'attr_equals'"
				v-model:value="cond.value"
				placeholder="Value, e.g. whitening"
				class="w-180px" />
		</template>
	</div>
</template>

<script lang="ts" setup>
import type { SelectOption } from 'naive-ui'
import type { WorkflowCond } from '@/api/modules/workflow'
import { conditionOptions } from '../presets'

const cond = defineModel<WorkflowCond>('cond', { required: true })

const props = defineProps<{
	tagOptions: SelectOption[]
	campaignOptions: SelectOption[]
	/** Exit conditions are checked before any email, so "last email" makes no sense there. */
	noLastEmail?: boolean
}>()

const options = computed(() =>
	props.noLastEmail ? conditionOptions.filter(o => !o.value.endsWith('_last_email')) : conditionOptions
)
</script>
