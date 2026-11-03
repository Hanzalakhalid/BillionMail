import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
  },
}))

import { getDeliveryLogs } from './delivery-log'

describe('delivery log API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getDeliveryLogs calls GET /delivery_log/list with params', () => {
    const params = { status: 'bounced', keyword: 'gmail', page: 1, page_size: 20 }
    getDeliveryLogs(params)
    expect(mockGet).toHaveBeenCalledWith('/delivery_log/list', { params })
  })
})
