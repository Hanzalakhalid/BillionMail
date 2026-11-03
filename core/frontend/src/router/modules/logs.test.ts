import { describe, it, expect, vi } from 'vitest'

vi.mock('@/router/constant', () => ({
  Layout: () => Promise.resolve({ template: '<router-view />' }),
}))

import route from './logs'

describe('logs route', () => {
  it('has correct path and redirect', () => {
    expect(route.path).toBe('/logs')
    expect(route.redirect).toBe('/logs/operation')
  })

  it('has correct meta', () => {
    expect(route.meta).toMatchObject({
      key: 'logs',
      title: 'Logs',
      titleKey: 'layout.menu.logs',
    })
  })

  it('first child has operation, error and delivery sub-routes', () => {
    const logs = route.children![0]
    expect(logs.name).toBe('Logs')
    expect(logs.redirect).toBe('/logs/operation')
    const subRoutes = logs.children!
    expect(subRoutes).toHaveLength(3)

    expect(subRoutes[2].path).toBe('delivery')
    expect(subRoutes[2].name).toBe('Delivery Logs')

    expect(subRoutes[0].path).toBe('operation')
    expect(subRoutes[0].name).toBe('Operation Logs')

    expect(subRoutes[1].path).toBe('error')
    expect(subRoutes[1].name).toBe('Error Logs')
  })
})
