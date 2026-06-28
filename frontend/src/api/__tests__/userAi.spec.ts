import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.hoisted(() => vi.fn())
const patch = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
    patch,
  },
}))

describe('user AI image API', () => {
  beforeEach(() => {
    post.mockReset()
    patch.mockReset()
    post.mockResolvedValue({
      data: {
        created: 1712345678,
        data: [],
      },
    })
    patch.mockResolvedValue({
      data: {
        id: 42,
        user_id: 7,
        group_id: 12,
        title: '首尔服务器部署',
        title_generated: true,
        model: 'gpt-4o',
        created_at: '2026-06-28T00:00:00Z',
        updated_at: '2026-06-28T00:00:00Z',
        messages: [],
      },
    })
  })

  it('posts image edit requests to the plural edits endpoint', async () => {
    const { userAiAPI, USER_IMAGE_EDITS_ENDPOINT } = await import('@/api/userAi')
    const abortController = new AbortController()
    const payload = {
      prompt: 'make it brighter',
      model: 'gpt-image-1',
      image_urls: ['https://cdn.example.com/input.png'],
    }

    await userAiAPI.editImages(payload, {
      signal: abortController.signal,
    })

    expect(USER_IMAGE_EDITS_ENDPOINT).toBe('/user/images/edits')
    expect(post).toHaveBeenCalledWith('/user/images/edits', payload, {
      timeout: 180000,
      signal: abortController.signal,
    })
    expect(post).not.toHaveBeenCalledWith('/user/images/edit', expect.anything(), expect.anything())
  })

  it('updates generated conversation titles through the title endpoint', async () => {
    const { userAiAPI } = await import('@/api/userAi')

    const updated = await userAiAPI.updateConversationTitle(42, { title: '首尔服务器部署' })

    expect(patch).toHaveBeenCalledWith('/user/chat/conversations/42/title', { title: '首尔服务器部署' })
    expect(updated.title).toBe('首尔服务器部署')
    expect(updated.title_generated).toBe(true)
  })
})
