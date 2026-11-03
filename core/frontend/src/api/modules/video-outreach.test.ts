import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockPost = vi.fn()

vi.mock('@/api', () => ({
  instance: {
    get: (...args: any[]) => mockGet(...args),
    post: (...args: any[]) => mockPost(...args),
  },
}))

import { listVideoOutreach, importLeads, setVideo } from './video-outreach'

describe('video outreach API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('listVideoOutreach calls GET /video_outreach/list with params', () => {
    const params = { tier: 'tier_1', page: 1, page_size: 20 }
    listVideoOutreach(params)
    expect(mockGet).toHaveBeenCalledWith('/video_outreach/list', { params })
  })

  it('importLeads calls POST /video_outreach/import_leads', () => {
    const data = { group_id: 2, content: 'email\na@b.com' }
    importLeads(data)
    expect(mockPost).toHaveBeenCalledWith('/video_outreach/import_leads', data, expect.any(Object))
  })

  it('setVideo calls POST /video_outreach/set_video', () => {
    const data = { contact_email: 'a@b.com', group_id: 2, video_url: 'https://v.example/a.mp4' }
    setVideo(data)
    expect(mockPost).toHaveBeenCalledWith('/video_outreach/set_video', data, expect.any(Object))
  })
})
