import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockPost = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
    post: (...args: any[]) => mockPost(...args),
  },
}))

import { getPushConfig, savePushConfig, sendPush, getPushLogs } from './push'

describe('push API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getPushConfig calls GET /push/config', () => {
    getPushConfig()
    expect(mockGet).toHaveBeenCalledWith('/push/config')
  })

  it('savePushConfig calls POST /push/config with a success message', () => {
    const data = { enabled: true, app_id: 'a', rest_api_key: 'k', default_url: '' }
    savePushConfig(data)
    expect(mockPost).toHaveBeenCalledWith(
      '/push/config',
      data,
      expect.objectContaining({
        fetchOptions: expect.objectContaining({ successMessage: true }),
      })
    )
  })

  it('sendPush calls POST /push/send', () => {
    const data = { audience: 'group' as const, group_id: 2, title: 'Sale', message: 'Hi', url: '', image: '' }
    sendPush(data)
    expect(mockPost).toHaveBeenCalledWith('/push/send', data, expect.any(Object))
  })

  it('getPushLogs calls GET /push/logs with params', () => {
    const params = { page: 1, page_size: 20, status: 'sent' }
    getPushLogs(params)
    expect(mockGet).toHaveBeenCalledWith('/push/logs', { params })
  })
})
