import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockPost = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
    post: (...args: any[]) => mockPost(...args),
  },
}))

import { getSmsConfig, saveSmsConfig, sendTestSms, sendGroupSms, getSmsLogs } from './sms'

describe('sms API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getSmsConfig calls GET /sms/config', () => {
    getSmsConfig()
    expect(mockGet).toHaveBeenCalledWith('/sms/config')
  })

  it('saveSmsConfig calls POST /sms/config with a success message', () => {
    const data = { enabled: true, provider: 'generic' } as any
    saveSmsConfig(data)
    expect(mockPost).toHaveBeenCalledWith(
      '/sms/config',
      data,
      expect.objectContaining({
        fetchOptions: expect.objectContaining({ successMessage: true }),
      })
    )
  })

  it('sendTestSms calls POST /sms/send_test', () => {
    const data = { phone: '03001234567', message: 'hi' }
    sendTestSms(data)
    expect(mockPost).toHaveBeenCalledWith('/sms/send_test', data, expect.any(Object))
  })

  it('sendGroupSms calls POST /sms/send_group', () => {
    const data = { group_id: 3, message: 'Hi {name}' }
    sendGroupSms(data)
    expect(mockPost).toHaveBeenCalledWith('/sms/send_group', data, expect.any(Object))
  })

  it('getSmsLogs calls GET /sms/logs with params', () => {
    const params = { page: 1, page_size: 20, status: 'failed' }
    getSmsLogs(params)
    expect(mockGet).toHaveBeenCalledWith('/sms/logs', { params })
  })
})
