import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockPost = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
    post: (...args: any[]) => mockPost(...args),
  },
}))

import { getAdFinderConfig, saveAdFinderConfig, searchAds, listAdvertisers, addAdvertiserLead } from './ad-finder'

describe('ad finder API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getAdFinderConfig calls GET /ad_intel/config', () => {
    getAdFinderConfig()
    expect(mockGet).toHaveBeenCalledWith('/ad_intel/config')
  })

  it('saveAdFinderConfig calls POST /ad_intel/config', () => {
    const data = { apify_token: 't', default_country: 'PK', results_limit: 100 }
    saveAdFinderConfig(data)
    expect(mockPost).toHaveBeenCalledWith('/ad_intel/config', data, expect.any(Object))
  })

  it('searchAds calls POST /ad_intel/search', () => {
    const data = { keyword: 'dental Karachi', country: 'PK', limit: 50 }
    searchAds(data)
    expect(mockPost).toHaveBeenCalledWith('/ad_intel/search', data, expect.any(Object))
  })

  it('listAdvertisers calls GET /ad_intel/list with params', () => {
    const params = { keyword: 'dental Karachi', page: 1, page_size: 20 }
    listAdvertisers(params)
    expect(mockGet).toHaveBeenCalledWith('/ad_intel/list', { params })
  })

  it('addAdvertiserLead calls POST /ad_intel/add_lead', () => {
    const data = { page_id: '123', group_id: 2 }
    addAdvertiserLead(data)
    expect(mockPost).toHaveBeenCalledWith('/ad_intel/add_lead', data, expect.any(Object))
  })
})
