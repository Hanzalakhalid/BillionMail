<template>
	<div class="palette">
		<div class="palette-head">
			<div class="palette-title">Actions</div>
			<div class="palette-sub">Drag onto the flow, or click to add {{ hint }}</div>
		</div>
		<div v-for="g in paletteGroups" :key="g.title" class="group">
			<div class="group-title">{{ g.title }}</div>
			<div
				v-for="type in g.types"
				:key="type"
				class="item"
				:class="`cat-${stepCategory(type)}`"
				draggable="true"
				@dragstart="onDragStart($event, type)"
				@dragend="ctx.dragType.value = null"
				@click="emit('add', type)">
				<span class="item-icon"><i :class="stepIcon(type)"></i></span>
				<span>{{ stepLabel(type) }}</span>
				<i class="i-mdi-drag-vertical drag"></i>
			</div>
		</div>
	</div>
</template>

<script lang="ts" setup>
import type { StepType } from '@/api/modules/workflow'
import { paletteGroups, stepCategory, stepIcon, stepLabel } from '../presets'
import { useBuilder } from '../builder'

defineProps<{ hint: string }>()

const emit = defineEmits<{ add: [type: StepType] }>()

const ctx = useBuilder()

const onDragStart = (e: DragEvent, type: StepType) => {
	ctx.dragType.value = type
	e.dataTransfer?.setData('text/plain', type)
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copy'
}
</script>

<style lang="scss" scoped>
.palette {
	padding: 16px 12px;
}

.palette-head {
	margin-bottom: 12px;
}

.palette-title {
	font-size: 15px;
	font-weight: 600;
}

.palette-sub {
	margin-top: 2px;
	font-size: 12px;
	color: var(--color-text-3, #888);
}

.group {
	margin-bottom: 14px;
}

.group-title {
	margin-bottom: 6px;
	font-size: 11px;
	font-weight: 700;
	color: var(--color-text-3, #888);
	text-transform: uppercase;
	letter-spacing: 0.5px;
}

.item {
	--accent: #2080f0;

	display: flex;
	gap: 10px;
	align-items: center;
	padding: 8px 10px;
	margin-bottom: 6px;
	font-size: 13px;
	cursor: grab;
	user-select: none;
	background: var(--color-bg-2, #fff);
	border: 1px solid var(--color-border-2, #e5e6eb);
	border-radius: 8px;
	transition:
		border-color 0.15s,
		box-shadow 0.15s;

	&:hover {
		border-color: var(--accent);
		box-shadow: 0 2px 8px rgb(0 0 0 / 6%);

		.drag {
			opacity: 1;
		}
	}

	&:active {
		cursor: grabbing;
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

.item-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 26px;
	height: 26px;
	font-size: 15px;
	color: var(--accent);
	background: color-mix(in srgb, var(--accent) 12%, transparent);
	border-radius: 6px;
}

.drag {
	margin-left: auto;
	color: var(--color-text-3, #888);
	opacity: 0;
}
</style>
