import { instance } from '@/api'

export type TriggerType = 'group_join' | 'tag_added' | 'date' | 'email_opened' | 'email_clicked'

export type StepType =
	| 'wait'
	| 'email'
	| 'sms'
	| 'whatsapp'
	| 'notify'
	| 'add_tag'
	| 'remove_tag'
	| 'set_attribute'
	| 'condition'
	| 'split'
	| 'webhook'
	| 'add_to_group'
	| 'remove_attribute'

/** Klaviyo-style message status: live sends, manual waits for review, draft is skipped. */
export type MessageStatus = 'live' | 'manual' | 'draft'

export type ConditionType =
	| 'has_tag'
	| 'not_has_tag'
	| 'has_phone'
	| 'subscribed'
	| 'attr_equals'
	| 'attr_set'
	| 'opened_campaign'
	| 'clicked_campaign'
	| 'opened_last_email'
	| 'clicked_last_email'

/** A check on a contact: used by conditions, Yes/No splits and exit conditions. */
export interface WorkflowCond {
	condition?: ConditionType
	tag_id?: number
	attribute?: string
	value?: string
	task_id?: number
}

export interface WorkflowTrigger {
	type: TriggerType
	group_id: number
	tag_id: number
	task_id: number
	attribute: string
	offset_days: number
	yearly: boolean
	monthly: boolean
	/** always (default), once, or after reentry_days */
	reentry: 'always' | 'once' | 'after'
	reentry_days: number
	include_existing: boolean
}

export interface WorkflowStep extends WorkflowCond {
	type: StepType
	wait_value?: number
	wait_unit?: 'minutes' | 'hours' | 'days'
	at_time?: string
	weekdays?: number[]
	timezone?: string
	status?: MessageStatus
	filters?: WorkflowCond[]
	skip_smart_sending?: boolean
	webhook_url?: string
	webhook_headers?: string
	group_id?: number
	template_id?: number
	subject?: string
	addresser?: string
	full_name?: string
	message?: string
	wa_template?: string
	wa_language?: string
	wa_params?: string[]
	notify_channel?: 'email' | 'sms'
	notify_to?: string
	split_mode?: 'condition' | 'random'
	percent?: number
	yes?: WorkflowStep[]
	no?: WorkflowStep[]
}

export interface WorkflowSettings {
	quiet_hours: { enabled: boolean; start: number; end: number; timezone: string }
	smart_sending_hours: number
	exit_conditions: WorkflowCond[]
	/** All must be true: checked on entry and before every message */
	flow_filters: WorkflowCond[]
}

export interface WorkflowStepStats {
	done: number
	skipped: number
	failed: number
	exited: number
	review: number
	waiting: number
	opened: number
	clicked: number
}

export interface WorkflowStats {
	total: number
	active: number
	completed: number
	exited: number
	skipped: number
}

export interface Workflow {
	id: number
	name: string
	status: 'draft' | 'active' | 'paused'
	trigger: WorkflowTrigger
	steps: WorkflowStep[]
	settings: WorkflowSettings
	enroll_since: number
	create_time: number
	update_time: number
	stats?: WorkflowStats
}

export interface WorkflowRun {
	id: number
	workflow_id: number
	contact_id: number
	email: string
	trigger_key: string
	step_index: number
	step_path: string
	status: 'active' | 'completed' | 'exited' | 'skipped'
	next_run_at: number
	last_error: string
	create_time: number
	update_time: number
}

export interface WorkflowLog {
	id: number
	workflow_id: number
	run_id: number
	contact_id: number
	email: string
	step_index: number
	step_path: string
	step_type: StepType
	status: 'done' | 'skipped' | 'failed' | 'exited' | 'waiting' | 'review'
	detail: string
	message_id: string
	create_time: number
}

export interface WorkflowPayload {
	name: string
	trigger: WorkflowTrigger
	steps: WorkflowStep[]
	settings: WorkflowSettings
}

export const getWorkflowList = () => {
	return instance.get('/workflow/list')
}

export const getWorkflowDetail = (id: number) => {
	return instance.get('/workflow/detail', { params: { id } })
}

export const createWorkflow = (data: WorkflowPayload) => {
	return instance.post('/workflow/create', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const updateWorkflow = (data: WorkflowPayload & { id: number }) => {
	return instance.post('/workflow/update', data, {
		fetchOptions: { loading: 'Saving...', successMessage: true },
	})
}

export const deleteWorkflow = (id: number) => {
	return instance.post(
		'/workflow/delete',
		{ id },
		{ fetchOptions: { loading: 'Deleting...', successMessage: true } }
	)
}

export const setWorkflowStatus = (id: number, status: 'active' | 'paused') => {
	return instance.post(
		'/workflow/set_status',
		{ id, status },
		{ fetchOptions: { loading: 'Saving...', successMessage: true } }
	)
}

export const getWorkflowRuns = (params: {
	workflow_id: number
	status?: string | null
	keyword?: string
	page: number
	page_size: number
}) => {
	return instance.get('/workflow/runs', { params })
}

export const getWorkflowLogs = (params: {
	workflow_id: number
	status?: string | null
	page: number
	page_size: number
}) => {
	return instance.get('/workflow/logs', { params })
}

/** Per-step report keyed by step path ("0", "2/y/0", ...). */
export const getWorkflowStepStats = (workflowId: number) => {
	return instance.get('/workflow/step_stats', { params: { workflow_id: workflowId } })
}

export interface WorkflowReview {
	id: number
	workflow_id: number
	run_id: number
	contact_id: number
	email: string
	step_path: string
	step_type: StepType
	status: 'pending' | 'sent' | 'failed' | 'discarded'
	detail: string
	create_time: number
	resolve_time: number
}

/** Manual messages waiting for review ("Needs review" queue). */
export const getWorkflowReviews = (params: {
	workflow_id: number
	status?: string | null
	page: number
	page_size: number
}) => {
	return instance.get('/workflow/reviews', { params })
}

export const resolveWorkflowReviews = (data: { workflow_id: number; ids: number[]; action: 'send' | 'discard' }) => {
	return instance.post('/workflow/resolve_reviews', data, {
		fetchOptions: { loading: data.action === 'send' ? 'Sending...' : 'Discarding...', successMessage: true },
	})
}
