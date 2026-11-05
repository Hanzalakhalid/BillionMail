import type {
	StepType,
	WorkflowCond,
	WorkflowPayload,
	WorkflowSettings,
	WorkflowStep,
	WorkflowTrigger,
} from '@/api/modules/workflow'

export interface WorkflowPreset {
	key: string
	title: string
	description: string
	icon: string
	payload: WorkflowPayload
}

export const emptyTrigger = (): WorkflowTrigger => ({
	type: 'group_join',
	group_id: 0,
	tag_id: 0,
	task_id: 0,
	attribute: '',
	offset_days: 0,
	yearly: false,
	monthly: false,
	reentry: 'always',
	reentry_days: 30,
	include_existing: false,
})

export const emptySettings = (): WorkflowSettings => ({
	quiet_hours: { enabled: true, start: 9, end: 21, timezone: 'Asia/Dubai' },
	smart_sending_hours: 0,
	exit_conditions: [],
	flow_filters: [],
})

export const emptyPayload = (): WorkflowPayload => ({
	name: '',
	trigger: emptyTrigger(),
	steps: [],
	settings: emptySettings(),
})

/** A new step of the given type with sensible defaults. */
export const newStep = (type: StepType): WorkflowStep => {
	switch (type) {
		case 'wait':
			return { type, wait_value: 1, wait_unit: 'days' }
		case 'email':
			return { type, template_id: undefined, subject: '', addresser: '', full_name: '' }
		case 'sms':
			return { type, message: '' }
		case 'whatsapp':
			return { type, wa_template: '', wa_language: 'en', wa_params: [] }
		case 'notify':
			return {
				type,
				notify_channel: 'email',
				notify_to: '',
				subject: '',
				addresser: '',
				full_name: '',
				message: 'New lead: {name} ({email}, {phone}). Please call them today.',
			}
		case 'add_tag':
		case 'remove_tag':
			return { type, tag_id: undefined }
		case 'set_attribute':
			return { type, attribute: '', value: '{today}' }
		case 'condition':
			return { type, condition: 'has_phone' }
		case 'split':
			return { type, split_mode: 'condition', condition: 'opened_last_email', percent: 50, yes: [], no: [] }
		case 'webhook':
			return { type, webhook_url: '', webhook_headers: '' }
		case 'add_to_group':
			return { type, group_id: undefined }
		case 'remove_attribute':
			return { type, attribute: '' }
	}
}

export const stepTypeOptions: { label: string; value: StepType; icon: string }[] = [
	{ label: 'Wait', value: 'wait', icon: 'i-mdi-timer-sand' },
	{ label: 'Send email', value: 'email', icon: 'i-mdi-email-outline' },
	{ label: 'Send WhatsApp', value: 'whatsapp', icon: 'i-mdi-whatsapp' },
	{ label: 'Send SMS', value: 'sms', icon: 'i-mdi-message-text-outline' },
	{ label: 'Alert staff', value: 'notify', icon: 'i-mdi-bell-alert-outline' },
	{ label: 'Yes/No or A/B split', value: 'split', icon: 'i-mdi-call-split' },
	{ label: 'Only continue if...', value: 'condition', icon: 'i-mdi-filter-outline' },
	{ label: 'Add tag', value: 'add_tag', icon: 'i-mdi-tag-plus-outline' },
	{ label: 'Remove tag', value: 'remove_tag', icon: 'i-mdi-tag-remove-outline' },
	{ label: 'Update a field', value: 'set_attribute', icon: 'i-mdi-form-textbox' },
	{ label: 'Remove a field', value: 'remove_attribute', icon: 'i-mdi-form-textbox-password' },
	{ label: 'Add to group', value: 'add_to_group', icon: 'i-mdi-account-multiple-plus-outline' },
	{ label: 'Webhook', value: 'webhook', icon: 'i-mdi-webhook' },
]

export const weekdayOptions = [
	{ label: 'Sun', value: 0 },
	{ label: 'Mon', value: 1 },
	{ label: 'Tue', value: 2 },
	{ label: 'Wed', value: 3 },
	{ label: 'Thu', value: 4 },
	{ label: 'Fri', value: 5 },
	{ label: 'Sat', value: 6 },
]

export const conditionOptions = [
	{ label: 'Opened the last email', value: 'opened_last_email' },
	{ label: 'Clicked the last email', value: 'clicked_last_email' },
	{ label: 'Has tag', value: 'has_tag' },
	{ label: "Doesn't have tag", value: 'not_has_tag' },
	{ label: 'Has a phone number', value: 'has_phone' },
	{ label: 'Is subscribed', value: 'subscribed' },
	{ label: 'Field equals', value: 'attr_equals' },
	{ label: 'Field is filled in', value: 'attr_set' },
	{ label: 'Opened a campaign', value: 'opened_campaign' },
	{ label: 'Clicked a campaign', value: 'clicked_campaign' },
]

export const isMessageStep = (type: StepType) => type === 'email' || type === 'sms' || type === 'whatsapp'

export const stepLabel = (type: StepType) => stepTypeOptions.find(o => o.value === type)?.label ?? type

export const stepIcon = (type: StepType) =>
	stepTypeOptions.find(o => o.value === type)?.icon ?? 'i-mdi-circle-outline'

export interface Lookups {
	groupName: (id: number) => string
	tagName: (id: number) => string
	templateName: (id: number) => string
	campaignName: (id: number) => string
}

const hostOf = (url: string) => {
	try {
		return new URL(url).host
	} catch {
		return url
	}
}

const plural = (n: number, word: string) => `${n} ${n === 1 ? word.replace(/s$/, '') : word}`

/** One-line, human readable description of a trigger. */
export const describeTrigger = (t: WorkflowTrigger, l: Lookups): string => {
	switch (t.type) {
		case 'group_join':
			return t.group_id ? `Contact joins group "${l.groupName(t.group_id)}"` : 'Contact joins a group'
		case 'tag_added':
			return t.tag_id ? `Tag "${l.tagName(t.tag_id)}" is added` : 'A tag is added'
		case 'email_opened':
			return t.task_id ? `Opens campaign "${l.campaignName(t.task_id)}"` : 'Opens a campaign email'
		case 'email_clicked':
			return t.task_id ? `Clicks a link in "${l.campaignName(t.task_id)}"` : 'Clicks a campaign link'
		case 'date': {
			const attr = t.attribute || 'a date'
			const scope = t.group_id ? ` (group "${l.groupName(t.group_id)}")` : ''
			let when = `On ${attr}`
			if (t.offset_days > 0) when = `${plural(t.offset_days, 'days')} after ${attr}`
			if (t.offset_days < 0) when = `${plural(-t.offset_days, 'days')} before ${attr}`
			const repeat = t.yearly ? ', every year' : t.monthly ? ', every month' : ''
			return `${when}${repeat}${scope}`
		}
	}
}

/** Describes a condition as "they ..." text. */
export const describeCond = (c: WorkflowCond, l: Lookups): string => {
	switch (c.condition) {
		case 'has_tag':
			return `have tag "${l.tagName(c.tag_id ?? 0)}"`
		case 'not_has_tag':
			return `don't have tag "${l.tagName(c.tag_id ?? 0)}"`
		case 'has_phone':
			return 'have a phone number'
		case 'subscribed':
			return 'are subscribed'
		case 'attr_equals':
			return `${c.attribute || 'field'} is "${c.value ?? ''}"`
		case 'attr_set':
			return `${c.attribute || 'field'} is filled in`
		case 'opened_campaign':
			return `opened "${l.campaignName(c.task_id ?? 0)}"`
		case 'clicked_campaign':
			return `clicked "${l.campaignName(c.task_id ?? 0)}"`
		case 'opened_last_email':
			return 'opened the last email'
		case 'clicked_last_email':
			return 'clicked the last email'
		default:
			return String(c.condition ?? '')
	}
}

/** One-line, human readable description of a step, with (Manual)/(Draft) for messages. */
export const describeStep = (s: WorkflowStep, l: Lookups): string => {
	const text = describeStepBody(s, l)
	if (isMessageStep(s.type) && s.status === 'manual') return `${text} (manual review)`
	if (isMessageStep(s.type) && s.status === 'draft') return `${text} (draft, not sent)`
	return text
}

const describeStepBody = (s: WorkflowStep, l: Lookups): string => {
	switch (s.type) {
		case 'wait': {
			let text = `Wait ${plural(s.wait_value ?? 0, s.wait_unit ?? 'days')}`
			if (s.at_time) text += `, then at ${s.at_time}`
			if (s.weekdays?.length && s.weekdays.length < 7)
				text += ` (${s.weekdays.map(d => weekdayOptions[d]?.label).join('/')} only)`
			return text
		}
		case 'email':
			return s.template_id
				? `Email "${s.subject || l.templateName(s.template_id)}"`
				: 'Email (choose a template)'
		case 'sms':
			return s.message
				? `SMS: ${s.message.slice(0, 60)}${s.message.length > 60 ? '…' : ''}`
				: 'SMS (write a message)'
		case 'whatsapp':
			return s.wa_template ? `WhatsApp template "${s.wa_template}"` : 'WhatsApp (choose a template)'
		case 'notify':
			return s.notify_to ? `Alert staff by ${s.notify_channel === 'sms' ? 'SMS' : 'email'}: ${s.notify_to}` : 'Alert staff'
		case 'add_tag':
			return s.tag_id ? `Add tag "${l.tagName(s.tag_id)}"` : 'Add a tag'
		case 'remove_tag':
			return s.tag_id ? `Remove tag "${l.tagName(s.tag_id)}"` : 'Remove a tag'
		case 'set_attribute':
			return s.attribute ? `Set ${s.attribute} to "${s.value ?? ''}"` : 'Update a field'
		case 'remove_attribute':
			return s.attribute ? `Remove field ${s.attribute}` : 'Remove a field'
		case 'add_to_group':
			return s.group_id ? `Add to group "${l.groupName(s.group_id)}"` : 'Add to a group'
		case 'webhook':
			return s.webhook_url ? `Webhook to ${hostOf(s.webhook_url)}` : 'Webhook (enter a URL)'
		case 'condition':
			return `Only if they ${describeCond(s, l)}`
		case 'split':
			return s.split_mode === 'random'
				? `A/B test: ${s.percent ?? 50}% A, ${100 - (s.percent ?? 50)}% B`
				: `If they ${describeCond(s, l)}`
	}
}

/** Finds the step a run path points at ("2/y/0" = first step of the Yes branch of step 3). */
export const stepAtPath = (steps: WorkflowStep[], path: string): WorkflowStep | undefined => {
	const parts = path ? path.split('/') : []
	if (!parts.length || parts.length % 2 === 0) return undefined
	let list: WorkflowStep[] | undefined = steps
	for (let i = 0; i < parts.length; i += 2) {
		const step: WorkflowStep | undefined = list?.[Number(parts[i])]
		if (!step) return undefined
		if (i === parts.length - 1) return step
		list = parts[i + 1] === 'y' ? step.yes : step.no
	}
	return undefined
}

/** Readable step number for a path: "3", "3 Yes > 1", "3 A > 2". */
export const pathLabel = (steps: WorkflowStep[], path: string): string => {
	const parts = path ? path.split('/') : []
	let list: WorkflowStep[] | undefined = steps
	const out: string[] = []
	for (let i = 0; i < parts.length; i += 2) {
		const idx = Number(parts[i])
		const step: WorkflowStep | undefined = list?.[idx]
		out.push(String(idx + 1))
		const branch = parts[i + 1]
		if (!branch) break
		const random = step?.split_mode === 'random'
		out.push(branch === 'y' ? (random ? 'A' : 'Yes') : random ? 'B' : 'No')
		list = branch === 'y' ? step?.yes : step?.no
	}
	return out.join(' ').replace(/ (Yes|No|A|B) /g, ' $1 > ')
}

/** Gives every split its yes/no arrays (the server leaves empty branches out). */
export const normalizeSteps = (steps: WorkflowStep[]): WorkflowStep[] => {
	for (const s of steps) {
		if (s.type === 'split') {
			s.yes = normalizeSteps(s.yes ?? [])
			s.no = normalizeSteps(s.no ?? [])
		}
	}
	return steps
}

/** Counts steps including those inside branches. */
export const countSteps = (steps: WorkflowStep[]): number =>
	steps.reduce((n, s) => n + 1 + (s.type === 'split' ? countSteps(s.yes ?? []) + countSteps(s.no ?? []) : 0), 0)

const sms = (message: string): WorkflowStep => ({ type: 'sms', message })

/** Ready-made flows for clinics. Groups, tags and templates are chosen after picking one. */
export const presets: WorkflowPreset[] = [
	{
		key: 'review',
		title: 'Ask for a Google review',
		description: 'After a visit, ask for a review. Stops as soon as they get the "Reviewed" tag.',
		icon: 'i-mdi-star-outline',
		payload: {
			name: 'Review request after visit',
			trigger: { ...emptyTrigger(), type: 'tag_added' },
			settings: { ...emptySettings(), exit_conditions: [{ condition: 'has_tag' }] },
			steps: [
				{ type: 'wait', wait_value: 2, wait_unit: 'hours' },
				{ type: 'condition', condition: 'has_phone' },
				sms(
					'Hi {name}, thank you for visiting us today! Could you share your experience on Google? It takes 30 seconds: [review link]'
				),
				{ type: 'wait', wait_value: 3, wait_unit: 'days' },
				sms('Hi {name}, a quick reminder: your Google review helps other patients find us. [review link] Thank you!'),
			],
		},
	},
	{
		key: 'recall',
		title: '6-month check-up reminder',
		description: 'Bring patients back 180 days after their last visit, with a follow-up a week later.',
		icon: 'i-mdi-calendar-refresh',
		payload: {
			name: '6-month check-up reminder',
			trigger: { ...emptyTrigger(), type: 'date', attribute: 'last_visit', offset_days: 180 },
			settings: emptySettings(),
			steps: [
				sms("Hi {name}, it's been 6 months since your last check-up. Book your cleaning this week: [booking link]"),
				{ type: 'wait', wait_value: 7, wait_unit: 'days' },
				sms('Hi {name}, we still have slots this week for your check-up. Reply to book.'),
			],
		},
	},
	{
		key: 'birthday',
		title: 'Birthday offer',
		description: "Send a birthday wish with an offer every year on the contact's birthday.",
		icon: 'i-mdi-cake-variant-outline',
		payload: {
			name: 'Birthday offer',
			trigger: { ...emptyTrigger(), type: 'date', attribute: 'birthday', yearly: true },
			settings: emptySettings(),
			steps: [sms('Happy birthday {name}! 🎉 Enjoy a free teeth cleaning with any treatment this month. Reply to book.')],
		},
	},
	{
		key: 'appointment',
		title: 'Appointment reminder',
		description: 'Remind patients one day before their appointment to reduce no-shows.',
		icon: 'i-mdi-bell-ring-outline',
		payload: {
			name: 'Appointment reminder',
			trigger: { ...emptyTrigger(), type: 'date', attribute: 'appointment_date', offset_days: -1 },
			settings: emptySettings(),
			steps: [
				sms('Hi {name}, a reminder of your appointment tomorrow. Reply YES to confirm or call us to reschedule.'),
			],
		},
	},
	{
		key: 'welcome',
		title: 'Welcome new leads',
		description:
			'Alert staff, send a welcome email, then WhatsApp/SMS only to people who did not open it.',
		icon: 'i-mdi-hand-wave-outline',
		payload: {
			name: 'Welcome new leads',
			trigger: { ...emptyTrigger(), type: 'group_join' },
			settings: { ...emptySettings(), smart_sending_hours: 12 },
			steps: [
				newStep('notify'),
				{ type: 'email', template_id: undefined, subject: 'Welcome, {{ .Subscriber.name }}!', addresser: '', full_name: '' },
				{ type: 'wait', wait_value: 1, wait_unit: 'days' },
				{
					type: 'split',
					split_mode: 'condition',
					condition: 'opened_last_email',
					yes: [],
					no: [
						{ type: 'condition', condition: 'has_phone' },
						sms('Hi {name}, thanks for your interest! Any questions about our offer? Just reply here.'),
					],
				},
			],
		},
	},
	{
		key: 'abtest',
		title: 'A/B test two offers',
		description: 'Half the leads get offer A, half get offer B. Compare replies in the step report.',
		icon: 'i-mdi-ab-testing',
		payload: {
			name: 'A/B test: which offer works better',
			trigger: { ...emptyTrigger(), type: 'group_join' },
			settings: emptySettings(),
			steps: [
				{
					type: 'split',
					split_mode: 'random',
					percent: 50,
					yes: [sms('Hi {name}, this week only: teeth cleaning for AED 99. Reply YES to book.')],
					no: [sms('Hi {name}, this week only: FREE consultation + X-ray. Reply YES to book.')],
				},
			],
		},
	},
]

/** Deep copy so editing a flow never changes the preset itself. */
export const clonePayload = (p: WorkflowPayload): WorkflowPayload => JSON.parse(JSON.stringify(p))

const condProblem = (c: WorkflowCond): string => {
	if ((c.condition === 'has_tag' || c.condition === 'not_has_tag') && !c.tag_id) return 'choose the tag to check'
	if ((c.condition === 'attr_equals' || c.condition === 'attr_set') && !c.attribute?.trim())
		return 'enter the field to check'
	if ((c.condition === 'opened_campaign' || c.condition === 'clicked_campaign') && !c.task_id)
		return 'choose the email campaign'
	return ''
}

const stepProblem = (s: WorkflowStep): string => {
	switch (s.type) {
		case 'wait':
			if (!(s.wait_value !== undefined && s.wait_value >= 0)) return 'wait time is missing'
			if (!s.at_time && !((s.wait_value ?? 0) > 0)) return 'wait time must be more than 0'
			if ((s.at_time || s.weekdays?.length) && s.wait_unit !== 'days') return 'a time of day or weekdays need the wait in days'
			if (s.at_time && !/^([01]\d|2[0-3]):[0-5]\d$/.test(s.at_time)) return 'time of day must look like 10:00'
			if (s.weekdays && s.weekdays.length === 0) return 'pick at least one weekday'
			return ''
		case 'email':
			if (!s.template_id) return 'choose an email template'
			if (!s.subject?.trim()) return 'enter a subject'
			if (!s.addresser?.trim()) return 'choose the sender'
			return ''
		case 'sms':
			return s.message?.trim() ? '' : 'write the SMS text'
		case 'whatsapp':
			return s.wa_template?.trim() && s.wa_language?.trim() ? '' : 'choose a WhatsApp template'
		case 'notify':
			if (!s.notify_to?.split(/[,;\n]/).some(x => x.trim())) return 'enter who to alert'
			if (!s.message?.trim()) return 'write the alert message'
			if (s.notify_channel === 'email' && !s.addresser?.trim()) return 'choose the mailbox the alert is sent from'
			return ''
		case 'add_tag':
		case 'remove_tag':
			return s.tag_id ? '' : 'choose a tag'
		case 'set_attribute':
			if (!s.attribute?.trim()) return 'enter the field to update'
			return /[\s{}]/.test(s.attribute) ? 'field names cannot contain spaces or braces' : ''
		case 'remove_attribute':
			if (!s.attribute?.trim()) return 'enter the field to remove'
			return s.attribute === 'phone' || s.attribute === 'name' ? 'name and phone cannot be removed' : ''
		case 'add_to_group':
			return s.group_id ? '' : 'choose the group'
		case 'webhook':
			if (!/^https?:\/\//.test(s.webhook_url?.trim() ?? '')) return 'webhook URL must start with http:// or https://'
			if ((s.webhook_headers ?? '').split('\n').some(l => l.trim() && !/^[^\s:]+\s*:/.test(l.trim())))
				return 'webhook headers must look like Name: value'
			return ''
		case 'condition':
			return condProblem(s)
		case 'split':
			if (s.split_mode === 'random') {
				const p = s.percent ?? 0
				return p >= 1 && p <= 99 ? '' : 'A/B share for A must be 1-99%'
			}
			return condProblem(s)
	}
}

const messageProblem = (s: WorkflowStep): string => {
	if (!isMessageStep(s.type)) return ''
	for (const [i, f] of (s.filters ?? []).entries()) {
		const p = condProblem(f)
		if (p) return `message filter ${i + 1}: ${p}`
	}
	return ''
}

const listProblem = (steps: WorkflowStep[], prefix: string, depth: number): string => {
	for (const [i, s] of steps.entries()) {
		const label = `${prefix}${i + 1}`
		const p = stepProblem(s) || messageProblem(s)
		if (p) return `Step ${label}: ${p}`
		if (s.type === 'split') {
			if (depth + 1 > 3) return `Step ${label}: branches can be nested at most 3 levels`
			const [yes, no] = s.split_mode === 'random' ? ['A', 'B'] : ['Yes', 'No']
			const inner =
				listProblem(s.yes ?? [], `${label} ${yes} > `, depth + 1) ||
				listProblem(s.no ?? [], `${label} ${no} > `, depth + 1)
			if (inner) return inner
		}
	}
	return ''
}

/** Returns the first problem with the form, or '' when it can be saved. Mirrors the server checks. */
export const validatePayload = (p: WorkflowPayload): string => {
	if (!p.name.trim()) return 'Enter a workflow name'
	const t = p.trigger
	if (t.type === 'group_join' && !t.group_id) return 'Choose the group that starts this workflow'
	if (t.type === 'tag_added' && !t.tag_id) return 'Choose the tag that starts this workflow'
	if ((t.type === 'email_opened' || t.type === 'email_clicked') && !t.task_id) return 'Choose the email campaign'
	if (t.type === 'date' && !t.attribute.trim()) return 'Enter the attribute that holds the date'
	if (t.reentry === 'after' && !(t.reentry_days >= 1)) return 'Re-entry: choose after how many days'
	const q = p.settings.quiet_hours
	if (q.enabled && q.start === q.end) return 'Quiet hours: start and end must be different'
	for (const [i, c] of p.settings.exit_conditions.entries()) {
		const problem = condProblem(c)
		if (problem) return `Exit condition ${i + 1}: ${problem}`
	}
	for (const [i, c] of (p.settings.flow_filters ?? []).entries()) {
		if (c.condition === 'opened_last_email' || c.condition === 'clicked_last_email')
			return `Flow filter ${i + 1}: "last email" can only be used in a step`
		const problem = condProblem(c)
		if (problem) return `Flow filter ${i + 1}: ${problem}`
	}
	if (!p.steps.length) return 'Add at least one step'
	if (countSteps(p.steps) > 100) return 'A workflow can have at most 100 steps'
	return listProblem(p.steps, '', 0)
}

// ---------- Canvas helpers (Klaviyo-style builder) ----------

/** What a step still needs before it can be saved, or ''. */
export const stepSetupProblem = (s: WorkflowStep): string => stepProblem(s) || messageProblem(s)

export type StepCategory = 'message' | 'timing' | 'logic' | 'data'

export const stepCategory = (type: StepType): StepCategory => {
	if (isMessageStep(type)) return 'message'
	if (type === 'wait') return 'timing'
	if (type === 'split' || type === 'condition') return 'logic'
	return 'data'
}

/** Palette groups shown on the left of the builder. */
export const paletteGroups: { title: string; types: StepType[] }[] = [
	{ title: 'Messages', types: ['email', 'whatsapp', 'sms'] },
	{ title: 'Timing', types: ['wait'] },
	{ title: 'Logic', types: ['split', 'condition'] },
	{ title: 'Actions', types: ['notify', 'add_tag', 'remove_tag', 'set_attribute', 'remove_attribute', 'add_to_group', 'webhook'] },
]

/** The step list a list path points at: '' = top level, '2/y' = Yes branch of step 3. */
export const listAt = (steps: WorkflowStep[], listPath: string): WorkflowStep[] | undefined => {
	if (!listPath) return steps
	const parts = listPath.split('/')
	if (parts.length % 2 !== 0) return undefined
	let list: WorkflowStep[] | undefined = steps
	for (let i = 0; i < parts.length; i += 2) {
		const step: WorkflowStep | undefined = list?.[Number(parts[i])]
		if (!step || step.type !== 'split') return undefined
		list = parts[i + 1] === 'y' ? (step.yes ??= []) : (step.no ??= [])
	}
	return list
}

/** Splits a step path into its list path and index: '2/y/0' -> { listPath: '2/y', index: 0 }. */
export const parentOf = (path: string): { listPath: string; index: number } => {
	const parts = path.split('/')
	const index = Number(parts.pop())
	return { listPath: parts.join('/'), index }
}

export const joinPath = (listPath: string, index: number) => (listPath ? `${listPath}/${index}` : `${index}`)

/** Inserts a new step and returns its path. */
export const insertStep = (steps: WorkflowStep[], listPath: string, index: number, type: StepType): string => {
	const list = listAt(steps, listPath)
	if (!list) return ''
	const at = Math.max(0, Math.min(index, list.length))
	list.splice(at, 0, newStep(type))
	return joinPath(listPath, at)
}

/** Removes the step at path. */
export const removeStep = (steps: WorkflowStep[], path: string) => {
	const { listPath, index } = parentOf(path)
	listAt(steps, listPath)?.splice(index, 1)
}

/** Copies the step at path right below it and returns the copy's path. */
export const duplicateStep = (steps: WorkflowStep[], path: string): string => {
	const { listPath, index } = parentOf(path)
	const list = listAt(steps, listPath)
	if (!list?.[index]) return ''
	list.splice(index + 1, 0, JSON.parse(JSON.stringify(list[index])))
	return joinPath(listPath, index + 1)
}

/** Moves the step up (-1) or down (+1) inside its list and returns its new path. */
export const moveStep = (steps: WorkflowStep[], path: string, dir: number): string => {
	const { listPath, index } = parentOf(path)
	const list = listAt(steps, listPath)
	const to = index + dir
	if (!list || to < 0 || to >= list.length) return path
	const [item] = list.splice(index, 1)
	list.splice(to, 0, item)
	return joinPath(listPath, to)
}
