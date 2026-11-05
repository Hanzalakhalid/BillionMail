import type { InjectionKey, Ref } from 'vue'
import type { StepType, WorkflowStepStats } from '@/api/modules/workflow'
import type { Lookups } from './presets'

/** What is open in the right-hand panel: the trigger, the flow settings, or a step path. */
export const TRIGGER = 'trigger'
export const SETTINGS = 'settings'

export type NodeAction = 'up' | 'down' | 'duplicate' | 'delete'

/** Shared state of the flow builder, provided by the editor page to every canvas node. */
export interface BuilderContext {
	selected: Ref<string>
	select: (path: string) => void
	stats: Ref<Record<string, WorkflowStepStats>>
	lookups: Lookups
	/** Step type being dragged from the palette, or null. */
	dragType: Ref<StepType | null>
	add: (listPath: string, index: number, type: StepType) => void
	act: (path: string, action: NodeAction) => void
}

export const builderKey: InjectionKey<BuilderContext> = Symbol('workflow-builder')

export const useBuilder = () => {
	const ctx = inject(builderKey)
	if (!ctx) throw new Error('workflow builder context missing')
	return ctx
}
