import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

describe('user AI image API', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({
      data: {
        created: 1712345678,
        data: [],
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
})
