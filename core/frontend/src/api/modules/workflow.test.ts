import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockPost = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
    post: (...args: any[]) => mockPost(...args),
  },
}))

import {
  getWorkflowList,
  getWorkflowDetail,
  createWorkflow,
  updateWorkflow,
  deleteWorkflow,
  setWorkflowStatus,
  getWorkflowRuns,
  getWorkflowLogs,
  getWorkflowReviews,
  resolveWorkflowReviews,
} from './workflow'

const payload = {
  name: 'Review request',
  trigger: {
    type: 'tag_added' as const,
    group_id: 0,
    tag_id: 3,
    attribute: '',
    offset_days: 0,
    yearly: false,
    monthly: false,
    reentry: 'always' as const,
    reentry_days: 0,
    task_id: 0,
    include_existing: false,
  },
  steps: [{ type: 'wait' as const, wait_value: 1, wait_unit: 'days' as const }],
  settings: {
    quiet_hours: { enabled: false, start: 9, end: 21, timezone: 'Asia/Dubai' },
    smart_sending_hours: 0,
    exit_conditions: [],
    flow_filters: [],
  },
}

describe('workflow API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getWorkflowList calls GET /workflow/list', () => {
    getWorkflowList()
    expect(mockGet).toHaveBeenCalledWith('/workflow/list')
  })

  it('getWorkflowDetail passes the id as a query param', () => {
    getWorkflowDetail(5)
    expect(mockGet).toHaveBeenCalledWith('/workflow/detail', { params: { id: 5 } })
  })

  it('createWorkflow posts the payload with a success message', () => {
    createWorkflow(payload)
    expect(mockPost).toHaveBeenCalledWith(
      '/workflow/create',
      payload,
      expect.objectContaining({ fetchOptions: expect.objectContaining({ successMessage: true }) })
    )
  })

  it('updateWorkflow posts the id with the payload', () => {
    updateWorkflow({ ...payload, id: 2 })
    expect(mockPost).toHaveBeenCalledWith('/workflow/update', { ...payload, id: 2 }, expect.any(Object))
  })

  it('deleteWorkflow posts the id', () => {
    deleteWorkflow(4)
    expect(mockPost).toHaveBeenCalledWith('/workflow/delete', { id: 4 }, expect.any(Object))
  })

  it('setWorkflowStatus posts id and status', () => {
    setWorkflowStatus(4, 'active')
    expect(mockPost).toHaveBeenCalledWith('/workflow/set_status', { id: 4, status: 'active' }, expect.any(Object))
  })

  it('getWorkflowRuns passes filters as query params', () => {
    const params = { workflow_id: 1, status: 'active', keyword: 'a@b.com', page: 1, page_size: 20 }
    getWorkflowRuns(params)
    expect(mockGet).toHaveBeenCalledWith('/workflow/runs', { params })
  })

  it('getWorkflowLogs passes filters as query params', () => {
    const params = { workflow_id: 1, status: 'failed', page: 2, page_size: 20 }
    getWorkflowLogs(params)
    expect(mockGet).toHaveBeenCalledWith('/workflow/logs', { params })
  })

  it('getWorkflowReviews passes filters as query params', () => {
    const params = { workflow_id: 3, status: 'pending', page: 1, page_size: 20 }
    getWorkflowReviews(params)
    expect(mockGet).toHaveBeenCalledWith('/workflow/reviews', { params })
  })

  it('resolveWorkflowReviews posts ids and the action', () => {
    const data = { workflow_id: 3, ids: [1, 2], action: 'send' as const }
    resolveWorkflowReviews(data)
    expect(mockPost).toHaveBeenCalledWith('/workflow/resolve_reviews', data, expect.any(Object))
  })
})
