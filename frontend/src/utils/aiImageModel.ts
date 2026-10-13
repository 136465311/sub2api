export function isImageModel(model: string): boolean {
  let normalized = model.trim().toLowerCase()
  for (const prefix of ['xai/', 'x-ai/', 'grok/']) {
    if (normalized.startsWith(prefix)) {
      normalized = normalized.slice(prefix.length)
      break
    }
  }
  return normalized === 'gpt-image' || normalized.startsWith('gpt-image-') ||
    normalized === 'grok-image' || normalized.startsWith('grok-image-') ||
    normalized === 'grok-imagine' || normalized === 'grok-imagine-1' ||
    normalized === 'grok-imagine-edit' || normalized.startsWith('grok-imagine-image')
}
