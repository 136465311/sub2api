import { describe, expect, it } from 'vitest'
import { isImageModel } from '../aiImageModel'

describe('isImageModel', () => {
  it('recognizes Grok Imagine images in chat and excludes video models', () => {
    expect(isImageModel('gpt-image-2')).toBe(true)
    expect(isImageModel('grok-imagine-image-2.0')).toBe(true)
    expect(isImageModel('xai/grok-imagine-image-quality')).toBe(true)
    expect(isImageModel('grok-imagine-edit')).toBe(true)
    expect(isImageModel('grok-imagine-video')).toBe(false)
    expect(isImageModel('grok-4.6')).toBe(false)
  })
})
