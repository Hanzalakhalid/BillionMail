import { describe, it, expect } from 'vitest'
import {
  clonePayload,
  countSteps,
  describeStep,
  describeTrigger,
  emptyPayload,
  emptyTrigger,
  newStep,
  pathLabel,
  presets,
  stepAtPath,
  stepTypeOptions,
  validatePayload,
  listAt,
  parentOf,
  joinPath,
  insertStep,
  duplicateStep,
  moveStep,
  removeStep,
  paletteGroups,
  stepCategory,
  stepSetupProblem,
} from './presets'
import type { WorkflowStep } from '@/api/modules/workflow'

const lookups = {
  groupName: (id: number) => `Group ${id}`,
  tagName: (id: number) => `Tag ${id}`,
  templateName: (id: number) => `Template ${id}`,
  campaignName: (id: number) => `Campaign ${id}`,
}

const flow = (): WorkflowStep[] => [
  { type: 'wait', wait_value: 1, wait_unit: 'days' },
  {
    type: 'split',
    split_mode: 'condition',
    condition: 'has_phone',
    yes: [
      { type: 'sms', message: 'hi' },
      { type: 'split', split_mode: 'random', percent: 50, yes: [{ type: 'add_tag', tag_id: 1 }], no: [] },
    ],
    no: [{ type: 'sms', message: '' }],
  },
]

describe('automation presets', () => {
  it('every step type has a default', () => {
    for (const o of stepTypeOptions) {
      expect(newStep(o.value).type).toBe(o.value)
    }
  })

  it('date presets are complete; others ask for a group, tag or template first', () => {
    for (const p of presets) {
      const problem = validatePayload(p.payload)
      if (p.payload.trigger.type === 'date') {
        expect(problem).toBe('')
      } else {
        expect(problem).not.toBe('')
      }
    }
  })

  it('the A/B preset only needs a group', () => {
    const ab = clonePayload(presets.find(p => p.key === 'abtest')!.payload)
    ab.trigger.group_id = 1
    expect(validatePayload(ab)).toBe('')
  })

  it('clonePayload returns an independent copy', () => {
    const copy = clonePayload(presets[0].payload)
    copy.steps[0].wait_value = 99
    copy.settings.exit_conditions.push({ condition: 'has_phone' })
    expect(presets[0].payload.steps[0].wait_value).toBe(2)
    expect(presets[0].payload.settings.exit_conditions).toHaveLength(1)
  })
})

describe('paths', () => {
  it('finds steps inside branches', () => {
    expect(stepAtPath(flow(), '1/y/1/y/0')?.type).toBe('add_tag')
    expect(stepAtPath(flow(), '1/n/0')?.type).toBe('sms')
    expect(stepAtPath(flow(), '1/y')).toBeUndefined()
    expect(stepAtPath(flow(), '5')).toBeUndefined()
  })

  it('labels paths for people', () => {
    expect(pathLabel(flow(), '0')).toBe('1')
    expect(pathLabel(flow(), '1/n/0')).toBe('2 No > 1')
    expect(pathLabel(flow(), '1/y/1/y/0')).toBe('2 Yes > 2 A > 1')
  })

  it('counts steps in branches', () => {
    expect(countSteps(flow())).toBe(6)
  })
})

describe('describeTrigger', () => {
  it('describes group, tag and email triggers', () => {
    expect(describeTrigger({ ...emptyTrigger(), group_id: 2 }, lookups)).toBe('Contact joins group "Group 2"')
    expect(describeTrigger({ ...emptyTrigger(), type: 'tag_added', tag_id: 5 }, lookups)).toBe('Tag "Tag 5" is added')
    expect(describeTrigger({ ...emptyTrigger(), type: 'email_opened', task_id: 7 }, lookups)).toBe(
      'Opens campaign "Campaign 7"'
    )
  })

  it('describes date triggers before, after and yearly', () => {
    const date = { ...emptyTrigger(), type: 'date' as const, attribute: 'last_visit' }
    expect(describeTrigger({ ...date, offset_days: 180 }, lookups)).toBe('180 days after last_visit')
    expect(describeTrigger({ ...date, offset_days: -1 }, lookups)).toBe('1 day before last_visit')
    expect(describeTrigger({ ...date, attribute: 'birthday', yearly: true, group_id: 3 }, lookups)).toBe(
      'On birthday, every year (group "Group 3")'
    )
  })
})

describe('describeStep', () => {
  it('describes each step type', () => {
    expect(describeStep({ type: 'wait', wait_value: 1, wait_unit: 'days' }, lookups)).toBe('Wait 1 day')
    expect(describeStep({ type: 'wait', wait_value: 3, wait_unit: 'hours' }, lookups)).toBe('Wait 3 hours')
    expect(describeStep({ type: 'email', template_id: 4, subject: '' }, lookups)).toBe('Email "Template 4"')
    expect(describeStep({ type: 'whatsapp', wa_template: 'review' }, lookups)).toBe('WhatsApp template "review"')
    expect(describeStep({ type: 'add_tag', tag_id: 1 }, lookups)).toBe('Add tag "Tag 1"')
    expect(describeStep({ type: 'set_attribute', attribute: 'last_visit', value: '{today}' }, lookups)).toBe(
      'Set last_visit to "{today}"'
    )
    expect(describeStep({ type: 'notify', notify_channel: 'sms', notify_to: '050' }, lookups)).toBe(
      'Alert staff by SMS: 050'
    )
    expect(describeStep({ type: 'condition', condition: 'not_has_tag', tag_id: 2 }, lookups)).toBe(
      'Only if they don\'t have tag "Tag 2"'
    )
    expect(describeStep({ type: 'split', split_mode: 'condition', condition: 'opened_last_email' }, lookups)).toBe(
      'If they opened the last email'
    )
    expect(describeStep({ type: 'split', split_mode: 'random', percent: 30 }, lookups)).toBe(
      'A/B test: 30% A, 70% B'
    )
  })

  it('shortens long SMS text', () => {
    const text = 'x'.repeat(80)
    expect(describeStep({ type: 'sms', message: text }, lookups)).toBe(`SMS: ${'x'.repeat(60)}…`)
  })
})

describe('validatePayload', () => {
  const base = () => ({ ...emptyPayload(), name: 'Test', trigger: { ...emptyTrigger(), group_id: 1 } })

  it('names the first broken step, including inside branches', () => {
    expect(validatePayload({ ...base(), steps: [newStep('sms')] })).toBe('Step 1: write the SMS text')
    expect(validatePayload({ ...base(), steps: flow() })).toBe('Step 2 No > 1: write the SMS text')
  })

  it('accepts a complete flow', () => {
    const steps = flow()
    steps[1].no![0].message = 'ok'
    expect(validatePayload({ ...base(), steps })).toBe('')
  })

  it('checks settings', () => {
    const p = { ...base(), steps: [{ type: 'sms' as const, message: 'x' }] }
    p.settings.quiet_hours = { enabled: true, start: 9, end: 9, timezone: 'Asia/Dubai' }
    expect(validatePayload(p)).toBe('Quiet hours: start and end must be different')
    p.settings.quiet_hours.end = 21
    p.settings.exit_conditions = [{ condition: 'has_tag' }]
    expect(validatePayload(p)).toBe('Exit condition 1: choose the tag to check')
  })

  it('requires a name and steps', () => {
    expect(validatePayload({ ...base(), name: ' ' })).toBe('Enter a workflow name')
    expect(validatePayload(base())).toBe('Add at least one step')
  })
})

describe('Klaviyo-style options', () => {
  const base = () => ({ ...emptyPayload(), name: 'Test', trigger: { ...emptyTrigger(), group_id: 1 } })

  it('validates time-of-day and weekday waits', () => {
    const p = base()
    p.steps = [{ type: 'wait', wait_value: 0, wait_unit: 'days', at_time: '10:00' }]
    expect(validatePayload(p)).toBe('')
    p.steps = [{ type: 'wait', wait_value: 1, wait_unit: 'hours', at_time: '10:00' }]
    expect(validatePayload(p)).toBe('Step 1: a time of day or weekdays need the wait in days')
    p.steps = [{ type: 'wait', wait_value: 1, wait_unit: 'days', weekdays: [] }]
    expect(validatePayload(p)).toBe('Step 1: pick at least one weekday')
  })

  it('validates webhook, group and field steps', () => {
    const p = base()
    p.steps = [{ type: 'webhook', webhook_url: 'n8n.example.com' }]
    expect(validatePayload(p)).toBe('Step 1: webhook URL must start with http:// or https://')
    p.steps = [{ type: 'webhook', webhook_url: 'https://x.io/h', webhook_headers: 'bad header' }]
    expect(validatePayload(p)).toBe('Step 1: webhook headers must look like Name: value')
    p.steps = [{ type: 'add_to_group' }]
    expect(validatePayload(p)).toBe('Step 1: choose the group')
    p.steps = [{ type: 'remove_attribute', attribute: 'phone' }]
    expect(validatePayload(p)).toBe('Step 1: name and phone cannot be removed')
  })

  it('validates message filters, flow filters and re-entry', () => {
    const p = base()
    p.steps = [{ type: 'sms', message: 'hi', filters: [{ condition: 'has_tag' }] }]
    expect(validatePayload(p)).toBe('Step 1: message filter 1: choose the tag to check')
    p.steps = [{ type: 'sms', message: 'hi' }]
    p.settings.flow_filters = [{ condition: 'opened_last_email' }]
    expect(validatePayload(p)).toBe('Flow filter 1: "last email" can only be used in a step')
    p.settings.flow_filters = []
    p.trigger.reentry = 'after'
    p.trigger.reentry_days = 0
    expect(validatePayload(p)).toBe('Re-entry: choose after how many days')
  })

  it('describes manual/draft messages and the new steps', () => {
    expect(describeStep({ type: 'sms', message: 'hi', status: 'manual' }, lookups)).toBe('SMS: hi (manual review)')
    expect(describeStep({ type: 'sms', message: 'hi', status: 'draft' }, lookups)).toBe('SMS: hi (draft, not sent)')
    expect(describeStep({ type: 'wait', wait_value: 1, wait_unit: 'days', at_time: '10:00', weekdays: [1, 3] }, lookups)).toBe(
      'Wait 1 day, then at 10:00 (Mon/Wed only)'
    )
    expect(describeStep({ type: 'webhook', webhook_url: 'https://hooks.n8n.io/x' }, lookups)).toBe('Webhook to hooks.n8n.io')
    expect(describeStep({ type: 'add_to_group', group_id: 4 }, lookups)).toBe('Add to group "Group 4"')
  })
})

describe('canvas helpers', () => {
  it('finds lists and parents by path', () => {
    const steps = flow()
    expect(listAt(steps, '')).toBe(steps)
    expect(listAt(steps, '1/y')).toBe(steps[1].yes)
    expect(listAt(steps, '0/y')).toBeUndefined()
    expect(parentOf('1/y/1')).toEqual({ listPath: '1/y', index: 1 })
    expect(parentOf('3')).toEqual({ listPath: '', index: 3 })
    expect(joinPath('', 2)).toBe('2')
    expect(joinPath('1/n', 0)).toBe('1/n/0')
  })

  it('inserts, duplicates, moves and removes steps', () => {
    const steps = flow()
    expect(insertStep(steps, '1/n', 0, 'wait')).toBe('1/n/0')
    expect(steps[1].no![0].type).toBe('wait')
    expect(duplicateStep(steps, '1/n/0')).toBe('1/n/1')
    expect(steps[1].no!.map(s => s.type)).toEqual(['wait', 'wait', 'sms'])
    expect(moveStep(steps, '1/n/2', -1)).toBe('1/n/1')
    expect(steps[1].no![1].type).toBe('sms')
    expect(moveStep(steps, '0', -1)).toBe('0')
    removeStep(steps, '1/n/1')
    expect(steps[1].no!.map(s => s.type)).toEqual(['wait', 'wait'])
  })

  it('groups step types for the palette and colours', () => {
    const all = paletteGroups.flatMap(g => g.types)
    expect(new Set(all).size).toBe(stepTypeOptions.length)
    expect(stepCategory('email')).toBe('message')
    expect(stepCategory('wait')).toBe('timing')
    expect(stepCategory('split')).toBe('logic')
    expect(stepCategory('webhook')).toBe('data')
    expect(stepSetupProblem({ type: 'sms', message: '' })).toBe('write the SMS text')
  })
})
