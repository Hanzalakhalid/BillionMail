<template>
	<div
		class="drop-point"
		:class="{ dragging: !!ctx.dragType.value, over, last }"
		@dragover.prevent="over = true"
		@dragleave="over = false"
		@drop.prevent="onDrop">
		<div class="line"></div>
		<n-dropdown trigger="click" :options="options" placement="bottom" @select="(t: StepType) => ctx.add(listPath, index, t)">
			<button type="button" class="plus" :title="last ? 'Add a step here' : 'Insert a step here'">
				<i class="i-mdi-plus"></i>
			</button>
		</n-dropdown>
		<div v-if="ctx.dragType.value" class="drop-hint">Drop here</div>
		<div v-if="!last" class="line"></div>
	</div>
</template>

<script lang="ts" setup>
import type { StepType } from '@/api/modules/workflow'
import { stepTypeOptions } from '../presets'
import { useBuilder } from '../builder'

const props = defineProps<{
	listPath: string
	index: number
	depth: number
	last?: boolean
}>()

const ctx = useBuilder()
const over = ref(false)

// Branches deeper than 3 levels are not allowed, so no split at the deepest level
const options = computed(() =>
	stepTypeOptions
		.filter(o => o.value !== 'split' || props.depth < 3)
		.map(o => ({ label: o.label, key: o.value }))
)

const onDrop = () => {
	over.value = false
	const type = ctx.dragType.value
	if (!type || (type === 'split' && props.depth >= 3)) return
	ctx.add(props.listPath, props.index, type)
	ctx.dragType.value = null
}
</script>

<style lang="scss" scoped>
.drop-point {
	display: flex;
	flex-direction: column;
	align-items: center;
	min-height: 44px;
}

.line {
	flex: 1;
	width: 2px;
	min-height: 10px;
	background: var(--color-border-3, #c9cdd4);
}

.plus {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 22px;
	height: 22px;
	font-size: 14px;
	color: var(--color-text-3, #888);
	cursor: pointer;
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-3, #c9cdd4);
	border-radius: 50%;
	transition: all 0.15s;

	&:hover {
		color: #fff;
		background: var(--color-primary-1, #18a058);
		border-color: var(--color-primary-1, #18a058);
		transform: scale(1.1);
	}
}

.drop-hint {
	display: none;
}

.dragging {
	.plus {
		border-style: dashed;
		border-color: var(--color-primary-1, #18a058);
	}
}

.dragging.over {
	.plus {
		display: none;
	}

	.drop-hint {
		display: block;
		padding: 6px 18px;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-primary-1, #18a058);
		background: rgb(24 160 88 / 8%);
		border: 2px dashed var(--color-primary-1, #18a058);
		border-radius: 8px;
	}
}
</style>
