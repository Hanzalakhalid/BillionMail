<template>
	<div
		class="node"
		:class="[`cat-${category}`, { selected: ctx.selected.value === path, draft: isDraft, problem: !!problem }]"
		@click.stop="ctx.select(path)">
		<div class="node-icon"><i :class="stepIcon(step.type)"></i></div>
		<div class="node-body">
			<div class="node-title">
				{{ title }}
				<span v-if="isMessageStep(step.type) && step.status === 'manual'" class="chip chip-info">Manual</span>
				<span v-if="isDraft" class="chip">Draft</span>
			</div>
			<div class="node-desc">{{ description }}</div>
			<div v-if="problem" class="node-problem">
				<i class="i-mdi-alert-circle-outline"></i>
				{{ problem }}
			</div>
			<div v-else-if="statLine" class="node-stats">{{ statLine }}</div>
		</div>

		<div class="node-tools" @click.stop>
			<button type="button" title="Move up" :disabled="isFirst" @click="ctx.act(path, 'up')">
				<i class="i-mdi-arrow-up"></i>
			</button>
			<button type="button" title="Move down" :disabled="isLast" @click="ctx.act(path, 'down')">
				<i class="i-mdi-arrow-down"></i>
			</button>
			<button type="button" title="Duplicate" @click="ctx.act(path, 'duplicate')">
				<i class="i-mdi-content-copy"></i>
			</button>
			<button type="button" title="Delete" class="danger" @click="ctx.act(path, 'delete')">
				<i class="i-mdi-delete-outline"></i>
			</button>
		</div>
	</div>
</template>

<script lang="ts" setup>
import type { WorkflowStep } from '@/api/modules/workflow'
import { describeStep, isMessageStep, stepCategory, stepIcon, stepLabel, stepSetupProblem } from '../presets'
import { useBuilder } from '../builder'

const props = defineProps<{
	step: WorkflowStep
	path: string
	isFirst: boolean
	isLast: boolean
}>()

const ctx = useBuilder()

const category = computed(() => stepCategory(props.step.type))
const isDraft = computed(() => isMessageStep(props.step.type) && props.step.status === 'draft')
const problem = computed(() => stepSetupProblem(props.step))

const title = computed(() => {
	if (props.step.type === 'split') return props.step.split_mode === 'random' ? 'A/B split' : 'Conditional split'
	return stepLabel(props.step.type)
})

const description = computed(() => describeStep(props.step, ctx.lookups))

// One short line of numbers, like Klaviyo's analytics under each message
const statLine = computed(() => {
	const s = ctx.stats.value[props.path]
	if (!s) return ''
	const parts: string[] = []
	if (s.waiting) parts.push(`${s.waiting} waiting`)
	if (props.step.type === 'wait') {
		parts.push(`${s.done} passed`)
	} else if (props.step.type === 'email') {
		const pct = (n: number) => (s.done ? ` (${Math.round((n / s.done) * 100)}%)` : '')
		parts.push(`${s.done} sent`, `${s.opened} opened${pct(s.opened)}`, `${s.clicked} clicked${pct(s.clicked)}`)
	} else {
		parts.push(`${s.done} ${isMessageStep(props.step.type) || props.step.type === 'notify' ? 'sent' : 'done'}`)
	}
	if (s.review) parts.push(`${s.review} for review`)
	if (s.skipped) parts.push(`${s.skipped} skipped`)
	if (s.failed) parts.push(`${s.failed} failed`)
	return parts.join(' · ')
})
</script>

<style lang="scss" scoped>
.node {
	--accent: #2080f0;

	position: relative;
	display: flex;
	gap: 12px;
	align-items: flex-start;
	width: 340px;
	max-width: 100%;
	padding: 12px 14px;
	cursor: pointer;
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-left: 4px solid var(--accent);
	border-radius: 10px;
	box-shadow: 0 1px 3px rgb(0 0 0 / 6%);
	transition:
		box-shadow 0.15s,
		border-color 0.15s;

	&:hover {
		box-shadow: 0 4px 14px rgb(0 0 0 / 10%);

		.node-tools {
			opacity: 1;
		}
	}

	&.selected {
		border-color: var(--accent);
		box-shadow:
			0 0 0 3px color-mix(in srgb, var(--accent) 25%, transparent),
			0 4px 14px rgb(0 0 0 / 10%);
	}

	&.draft {
		opacity: 0.6;
		border-style: dashed;
	}

	&.problem {
		border-left-color: #d03050;
	}
}

.cat-message {
	--accent: #2080f0;
}

.cat-timing {
	--accent: #f0a020;
}

.cat-logic {
	--accent: #8a63d2;
}

.cat-data {
	--accent: #18a058;
}

.node-icon {
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
}

.node-body {
	flex: 1;
	min-width: 0;
}

.node-title {
	display: flex;
	gap: 6px;
	align-items: center;
	font-size: 14px;
	font-weight: 600;
}

.node-desc {
	margin-top: 2px;
	overflow: hidden;
	font-size: 12px;
	color: var(--color-text-3, #888);
	text-overflow: ellipsis;
	white-space: nowrap;
}

.node-problem {
	display: flex;
	gap: 4px;
	align-items: center;
	margin-top: 4px;
	font-size: 12px;
	color: #d03050;
}

.node-stats {
	margin-top: 4px;
	font-size: 12px;
	font-weight: 500;
	color: var(--accent);
}

.chip {
	padding: 0 6px;
	font-size: 10px;
	font-weight: 600;
	line-height: 16px;
	color: var(--color-text-3, #888);
	text-transform: uppercase;
	background: var(--color-fill-2, #f2f3f5);
	border-radius: 8px;
}

.chip-info {
	color: #2080f0;
	background: rgb(32 128 240 / 10%);
}

.node-tools {
	position: absolute;
	top: -14px;
	right: 8px;
	display: flex;
	gap: 2px;
	padding: 2px;
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 6px;
	box-shadow: 0 2px 6px rgb(0 0 0 / 8%);
	opacity: 0;
	transition: opacity 0.15s;

	button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 22px;
		color: var(--color-text-2, #555);
		cursor: pointer;
		background: transparent;
		border: none;
		border-radius: 4px;

		&:hover:not(:disabled) {
			background: var(--color-fill-2, #f2f3f5);
		}

		&:disabled {
			cursor: default;
			opacity: 0.3;
		}

		&.danger:hover {
			color: #d03050;
		}
	}
}
</style>
