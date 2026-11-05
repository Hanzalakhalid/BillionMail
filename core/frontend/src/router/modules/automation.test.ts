import { describe, it, expect, vi } from 'vitest'

vi.mock('@/router/constant', () => ({
  Layout: () => Promise.resolve({ template: '<router-view />' }),
}))

import route from './automation'

describe('automation route', () => {
  it('has correct path', () => {
    expect(route.path).toBe('/automation')
  })

  it('is shown in the menu as Workflows', () => {
    expect(route.meta).toMatchObject({
      sort: 3,
      key: 'automation',
      title: 'Workflows',
      titleKey: 'layout.menu.workflows',
      icon: 'i-mdi-refresh-auto',
    })
    expect(route.meta?.hidden).toBeFalsy()
  })

  it('has the list page and a hidden editor page', () => {
    expect(route.children).toHaveLength(2)
    expect(route.children![0].name).toBe('Automation')
    expect(route.children![0].path).toBe('/automation')
    expect(route.children![1].name).toBe('AutomationEdit')
    expect(route.children![1].path).toBe('/automation/edit')
    expect(route.children![1].meta?.hidden).toBe(true)
  })
})
