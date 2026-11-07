import { describe, it, expect, vi } from 'vitest'

// Mock all external dependencies before importing
vi.mock('@/router', () => ({
  default: { push: vi.fn() },
}))

vi.mock('@/store', () => ({
  useUserStore: vi.fn(() => ({
    login: { token: 'test-token' },
    resetLoginInfo: vi.fn(),
  })),
}))

vi.mock('@/utils', () => ({
  apiUrlPrefix: '/test-prefix',
  isObject: (val: unknown) => val !== null && typeof val === 'object' && !Array.isArray(val),
  Message: {
    error: vi.fn(),
    success: vi.fn(),
    loading: vi.fn(() => ({ close: vi.fn() })),
  },
}))

import { instance, clearPendingRequests } from './index'
import axios from 'axios'
import { useUserStore } from '@/store'

describe('API instance', () => {
  it('is an axios instance with expected defaults', () => {
    expect(instance.defaults.timeout).toBe(600000)
    expect(instance.defaults.headers['Content-Type']).toBe('application/json')
  })

  it('has request interceptors registered', () => {
    // Axios stores interceptors internally
    const reqInterceptors = (instance.interceptors.request as any).handlers
    expect(reqInterceptors.length).toBeGreaterThanOrEqual(3)
  })

  it('has response interceptors registered', () => {
    const resInterceptors = (instance.interceptors.response as any).handlers
    expect(resInterceptors.length).toBeGreaterThanOrEqual(1)
  })
})

describe('clearPendingRequests', () => {
  it('is a function', () => {
    expect(typeof clearPendingRequests).toBe('function')
  })

  it('does not throw when called with no pending requests', () => {
    expect(() => clearPendingRequests()).not.toThrow()
  })
})

describe('versioned resource responses', () => {
  const adapter = (data: unknown) => async (config: any) => ({ data, status: 200, statusText: 'OK', headers: {}, config })

  it('returns direct versioned resources when requested', async () => {
    const page = { items: [{ id: 'asset' }], next_cursor: '' }
    const response = await instance.get('/v1/assets/list', { adapter: adapter(page), fetchOptions: { rawResponse: true } })
    expect(response).toEqual(page)
  })

  it('keeps existing envelope responses compatible', async () => {
    const data = { id: 'legacy-resource' }
    const response = await instance.get('/legacy/list', { adapter: adapter({ code: 0, success: true, data }) })
    expect(response).toEqual(data)
  })

  it('rejects application errors on direct resource requests', async () => {
    const failure = { code: 500, success: false, msg: 'Upload rejected' }
    await expect(instance.get('/v1/assets/uploads', { adapter: adapter(failure), fetchOptions: { rawResponse: true } })).rejects.toEqual(failure)
  })
})

describe('request prefix retries', () => {
  it.each([
    { baseURL: '/api', browserURL: '/api/api/v1/business/profile', backendURL: '/api/v1/business/profile' },
    { baseURL: '', browserURL: '/api/v1/business/profile', backendURL: '/api/v1/business/profile' },
  ])('keeps the deployment prefix separate from the backend API prefix ($baseURL)', async ({ baseURL, browserURL, backendURL }) => {
    await instance.get('/v1/business/profile', {
      baseURL,
      adapter: async (config: any) => {
        const uri = instance.getUri(config)
        expect(uri).toBe(browserURL)
        expect(baseURL ? uri.replace(/^\/api/, '') : uri).toBe(backendURL)
        return { data: { code: 0, success: true, data: {} }, status: 200, statusText: 'OK', headers: {}, config }
      },
    })
  })

  it('does not add another API prefix when replaying a request', async () => {
    let dispatched: any
    const adapter = async (config: any) => {
      dispatched = config
      return { data: { code: 0, success: true, data: {} }, status: 200, statusText: 'OK', headers: {}, config }
    }
    await instance.get('/v1/products/list', { adapter })
    expect(dispatched.url).toBe('/api/v1/products/list')
    await instance.request(dispatched)
    expect(dispatched.url).toBe('/api/v1/products/list')
  })

  it('keeps the original route after refreshing an expired token', async () => {
    const store = { login: { token: 'expired', refresh_token: 'refresh' }, setLoginInfo: vi.fn(), resetLoginInfo: vi.fn() }
    vi.mocked(useUserStore).mockReturnValue(store as any)
    const refresh = vi.spyOn(axios, 'post').mockResolvedValue({ data: { success: true, data: { token: 'renewed', refreshToken: 'rotated', ttl: 900 } } })
    const urls: string[] = []
    try {
      const result = await instance.get('/v1/business/profile', {
        fetchOptions: { rawResponse: true },
        adapter: async (config: any) => {
          urls.push(config.url)
          return { data: urls.length === 1 ? { code: 401, success: false, msg: 'Expired' } : { profile: null }, status: 200, statusText: 'OK', headers: {}, config }
        },
      })
      expect(result).toEqual({ profile: null })
      expect(urls).toEqual(['/api/v1/business/profile', '/api/v1/business/profile'])
      expect(store.setLoginInfo).toHaveBeenCalled()
    } finally { refresh.mockRestore() }
  })
})
