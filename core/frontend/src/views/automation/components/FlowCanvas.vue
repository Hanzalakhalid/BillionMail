<template>
	<div class="canvas-list">
		<template v-for="(step, index) in steps" :key="keyOf(step)">
			<drop-point :list-path="listPath" :index="index" :depth="depth" />
			<node-card
				:step="step"
				:path="joinPath(listPath, index)"
				:is-first="index === 0"
				:is-last="index === steps.length - 1" />

			<!-- Branches: Yes/No or A/B side by side, like Klaviyo -->
			<div v-if="step.type === 'split'" class="branches">
				<div v-for="b in branchesOf(step)" :key="b.key" class="branch">
					<div class="branch-stem"></div>
					<span class="branch-label" :class="b.key === 'y' ? 'yes' : 'no'">{{ b.label }}</span>
					<flow-canvas
						:steps="b.list"
						:list-path="`${joinPath(listPath, index)}/${b.key}`"
						:depth="depth + 1" />
				</div>
			</div>
		</template>

		<drop-point :list-path="listPath" :index="steps.length" :depth="depth" last />
		<div v-if="depth > 0 && !steps.length" class="empty-branch">No steps, goes on</div>
	</div>
</template>

<script lang="ts" setup>
import type { WorkflowStep } from '@/api/modules/workflow'
import { joinPath } from '../presets'
import DropPoint from './DropPoint.vue'
import NodeCard from './NodeCard.vue'

defineProps<{
	steps: WorkflowStep[]
	listPath: string
	depth: number
}>()

// Stable keys per step object so nodes keep their state when steps move
const keys = new WeakMap<object, number>()
let seq = 0
const keyOf = (step: WorkflowStep) => {
	if (!keys.has(step)) keys.set(step, ++seq)
	return keys.get(step)!
}

const branchesOf = (step: WorkflowStep) => {
	const random = step.split_mode === 'random'
	return [
		{ key: 'y', label: random ? `A · ${step.percent ?? 50}%` : 'YES', list: step.yes ?? [] },
		{ key: 'n', label: random ? `B · ${100 - (step.percent ?? 50)}%` : 'NO', list: step.no ?? [] },
	]
}
</script>

<style lang="scss" scoped>
.canvas-list {
	display: flex;
	flex-direction: column;
	align-items: center;
}

.branches {
	position: relative;
	display: flex;
	gap: 28px;
	justify-content: center;
	padding-top: 18px;

	// Horizontal connector across the branch tops
	&::before {
		position: absolute;
		top: 0;
		right: 25%;
		left: 25%;
		height: 2px;
		content: '';
		background: var(--color-border-3, #c9cdd4);
	}

	// Stem from the split node down to the connector
	&::after {
		position: absolute;
		top: -2px;
		left: 50%;
		width: 2px;
		height: 20px;
		content: '';
		background: var(--color-border-3, #c9cdd4);
	}
}

.branch {
	position: relative;
	display: flex;
	flex-direction: column;
	align-items: center;
	min-width: 360px;
	padding-top: 8px;
}

.branch-stem {
	position: absolute;
	top: -18px;
	left: 50%;
	width: 2px;
	height: 26px;
	background: var(--color-border-3, #c9cdd4);
}

.branch-label {
	position: relative;
	z-index: 1;
	padding: 2px 12px;
	font-size: 11px;
	font-weight: 700;
	color: #fff;
	letter-spacing: 0.5px;
	border-radius: 10px;

	&.yes {
		background: #18a058;
	}

	&.no {
		background: #d03050;
	}
}

.empty-branch {
	font-size: 11px;
	color: var(--color-text-3, #888);
}
</style>
