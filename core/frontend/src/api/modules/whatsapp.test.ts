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
  getWhatsappConfig,
  saveWhatsappConfig,
  getWhatsappTemplates,
  sendWhatsappTest,
  sendWhatsappGroup,
  getWhatsappLogs,
} from './whatsapp'

describe('whatsapp API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getWhatsappConfig calls GET /whatsapp/config', () => {
    getWhatsappConfig()
    expect(mockGet).toHaveBeenCalledWith('/whatsapp/config')
  })

  it('saveWhatsappConfig calls POST /whatsapp/config with a success message', () => {
    const data = {
      enabled: true,
      phone_number_id: 'pn',
      business_account_id: '',
      access_token: 't',
      api_version: 'v23.0',
      default_country_code: '92',
    }
    saveWhatsappConfig(data)
    expect(mockPost).toHaveBeenCalledWith(
      '/whatsapp/config',
      data,
      expect.objectContaining({
        fetchOptions: expect.objectContaining({ successMessage: true }),
      })
    )
  })

  it('getWhatsappTemplates calls GET /whatsapp/templates', () => {
    getWhatsappTemplates()
    expect(mockGet).toHaveBeenCalledWith('/whatsapp/templates')
  })

  it('sendWhatsappTest calls POST /whatsapp/send_test', () => {
    const data = { phone: '03001234567', template: 'hello_world', language: 'en_US', params: [] }
    sendWhatsappTest(data)
    expect(mockPost).toHaveBeenCalledWith('/whatsapp/send_test', data, expect.any(Object))
  })

  it('sendWhatsappGroup calls POST /whatsapp/send_group', () => {
    const data = { group_id: 2, template: 'reminder', language: 'en', params: ['{name}'] }
    sendWhatsappGroup(data)
    expect(mockPost).toHaveBeenCalledWith('/whatsapp/send_group', data, expect.any(Object))
  })

  it('getWhatsappLogs calls GET /whatsapp/logs with params', () => {
    const params = { page: 1, page_size: 20, status: 'failed' }
    getWhatsappLogs(params)
    expect(mockGet).toHaveBeenCalledWith('/whatsapp/logs', { params })
  })
})
