import { apiClient } from './client'
import { getLocale } from '@/i18n'
import type { PaginatedResponse } from '@/types'

export interface AIModelGroup {
  id: number
  name: string
  platform: string
  models: string[]
}

export interface AIModelsResult {
  groups: AIModelGroup[]
  default_group_id?: number | null
  default_model?: string
}

export interface GenerateImagePayload {
  prompt: string
  model: string
  size?: string
  n?: number
  group_id?: number | null
  conversation_id?: number | null
}

export interface EditImagePayload extends GenerateImagePayload {
  image_urls: string[]
}

export interface AIImageGenerationImage {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface AIImageGenerationResponse {
  created: number
  data: AIImageGenerationImage[]
  size?: string
  model?: string
}

export interface AIImageHistoryItem {
  id: number
  user_id: number
  group_id: number | null
  prompt: string
  model: string
  size: string
  n: number
  images: string[]
  created_at: string
}

export interface AIChatMessage {
  id: number
  conversation_id: number
  user_id: number
  role: string
  content: string
  model: string
  created_at: string
  updated_at: string
}

export interface AIConversation {
  id: number
  user_id: number
  group_id: number | null
  title: string
  model: string
  created_at: string
  updated_at: string
  messages: AIChatMessage[]
}

export interface CreateConversationPayload {
  title?: string
  model?: string
  group_id?: number | null
}

export interface ChatImageContentPart {
  type: 'image_url'
  image_url: {
    url: string
  }
}

export interface ChatTextContentPart {
  type: 'text'
  text: string
}

export type ChatMessageContent = string | Array<ChatTextContentPart | ChatImageContentPart>

export interface ChatCompletionMessage {
  role: 'system' | 'user' | 'assistant'
  content: ChatMessageContent
}

export interface StreamChatCompletionPayload {
  model: string
  group_id?: number | null
  conversation_id?: number | null
  messages: ChatCompletionMessage[]
  stream?: boolean
}

export interface StreamChatCompletionOptions {
  signal?: AbortSignal
  onDelta?: (delta: string) => void
}

export interface GenerateImageOptions {
  signal?: AbortSignal
}

export interface UploadImageResult {
  image_url: string
}

type RawRecord = Record<string, any>
type ParsedSSEFrame = {
  content: string
  done: boolean
}

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/$/, '')

function toNumber(value: unknown, fallback = 0): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : fallback
}

function toNullableNumber(value: unknown): number | null {
  if (value === null || value === undefined) return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

function normalizeMessage(raw: RawRecord): AIChatMessage {
  return {
    id: toNumber(raw.id ?? raw.ID),
    conversation_id: toNumber(raw.conversation_id ?? raw.ConversationID),
    user_id: toNumber(raw.user_id ?? raw.UserID),
    role: String(raw.role ?? raw.Role ?? ''),
    content: String(raw.content ?? raw.Content ?? ''),
    model: String(raw.model ?? raw.Model ?? ''),
    created_at: String(raw.created_at ?? raw.CreatedAt ?? ''),
    updated_at: String(raw.updated_at ?? raw.UpdatedAt ?? '')
  }
}

function normalizeConversation(raw: RawRecord): AIConversation {
  const messages = raw.messages ?? raw.Messages ?? []
  return {
    id: toNumber(raw.id ?? raw.ID),
    user_id: toNumber(raw.user_id ?? raw.UserID),
    group_id: toNullableNumber(raw.group_id ?? raw.GroupID),
    title: String(raw.title ?? raw.Title ?? ''),
    model: String(raw.model ?? raw.Model ?? ''),
    created_at: String(raw.created_at ?? raw.CreatedAt ?? ''),
    updated_at: String(raw.updated_at ?? raw.UpdatedAt ?? ''),
    messages: Array.isArray(messages) ? messages.map((item) => normalizeMessage(item)) : []
  }
}

function toStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item ?? '')).filter((item) => item.trim()) : []
}

function normalizeImageHistory(raw: RawRecord): AIImageHistoryItem {
  return {
    id: toNumber(raw.id ?? raw.ID),
    user_id: toNumber(raw.user_id ?? raw.UserID),
    group_id: toNullableNumber(raw.group_id ?? raw.GroupID),
    prompt: String(raw.prompt ?? raw.Prompt ?? ''),
    model: String(raw.model ?? raw.Model ?? ''),
    size: String(raw.size ?? raw.Size ?? ''),
    n: toNumber(raw.n ?? raw.N, 1),
    images: toStringArray(raw.images ?? raw.Images),
    created_at: String(raw.created_at ?? raw.CreatedAt ?? '')
  }
}

function normalizeImageGenerationImage(raw: RawRecord): AIImageGenerationImage {
  const url = typeof raw.url === 'string' ? raw.url : ''
  const b64 = typeof raw.b64_json === 'string' ? raw.b64_json : ''
  return {
    ...(url ? { url } : {}),
    ...(b64 ? { b64_json: b64 } : {}),
    ...(typeof raw.revised_prompt === 'string' && raw.revised_prompt
      ? { revised_prompt: raw.revised_prompt }
      : {})
  }
}

function normalizeImageGenerationResponse(raw: RawRecord): AIImageGenerationResponse {
  const data = Array.isArray(raw?.data) ? raw.data.map((item) => normalizeImageGenerationImage(item as RawRecord)) : []
  return {
    created: toNumber(raw?.created ?? raw?.created_at),
    data,
    ...(typeof raw?.size === 'string' && raw.size ? { size: raw.size } : {}),
    ...(typeof raw?.model === 'string' && raw.model ? { model: raw.model } : {})
  }
}

function mergeModelLists(primary: AIModelsResult, secondary: AIModelsResult): AIModelsResult {
  const groupsById = new Map<number, AIModelGroup>()
  for (const source of [primary, secondary]) {
    for (const group of source.groups || []) {
      const existing = groupsById.get(group.id)
      if (!existing) {
        groupsById.set(group.id, {
          ...group,
          models: [...(group.models || [])]
        })
        continue
      }
      const seen = new Set(existing.models.map((model) => model.toLowerCase()))
      for (const model of group.models || []) {
        const key = model.toLowerCase()
        if (seen.has(key)) continue
        seen.add(key)
        existing.models.push(model)
      }
    }
  }
  return {
    groups: Array.from(groupsById.values()),
    default_group_id: primary.default_group_id ?? secondary.default_group_id ?? null,
    default_model: primary.default_model || secondary.default_model || ''
  }
}

function authHeaders(): HeadersInit {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
    'Accept-Language': getLocale()
  }
  const token = localStorage.getItem('auth_token')
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  return headers
}

function stringifyContent(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) {
    return value
      .map((item) => {
        if (typeof item === 'string') return item
        if (item && typeof item === 'object') {
          const record = item as RawRecord
          return String(record.text ?? record.content ?? '')
        }
        return ''
      })
      .join('')
  }
  return ''
}

function extractAssistantContent(payload: any): string {
  const choice = Array.isArray(payload?.choices) ? payload.choices[0] : null
  return (
    stringifyContent(choice?.delta?.content) ||
    stringifyContent(choice?.message?.content) ||
    stringifyContent(payload?.delta?.content) ||
    stringifyContent(payload?.delta?.text) ||
    stringifyContent(typeof payload?.delta === 'string' ? payload.delta : '') ||
    stringifyContent(payload?.message?.content) ||
    stringifyContent(payload?.output_text) ||
    stringifyContent(payload?.text) ||
    stringifyContent(payload?.content)
  )
}

async function readErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  if (!text) return response.statusText || `HTTP ${response.status}`
  try {
    const data = JSON.parse(text)
    return data?.message || data?.detail || data?.error || response.statusText || `HTTP ${response.status}`
  } catch {
    return text
  }
}

function parseSSEFrame(frame: string, onDelta?: (delta: string) => void): ParsedSSEFrame {
  let content = ''
  let done = false
  for (const line of frame.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed.startsWith('data:')) continue

    const data = trimmed.slice(5).trim()
    if (!data) continue
    if (data === '[DONE]') {
      done = true
      continue
    }

    try {
      const payload = JSON.parse(data)
      const delta = extractAssistantContent(payload)
      if (delta) {
        content += delta
        onDelta?.(delta)
      }
    } catch {
      // Ignore malformed stream frames; the backend may also emit event metadata.
    }
  }
  return { content, done }
}

export const userAiAPI = {
  async getModels(): Promise<AIModelsResult> {
    return apiClient.get('/user/ai/models').then((res) => res.data)
  },

  async getImageModels(): Promise<AIModelsResult> {
    return apiClient.get('/user/images/models').then((res) => res.data)
  },

  async getChatModelsWithImages(): Promise<AIModelsResult> {
    const chatModels = await this.getModels()
    try {
      const imageModels = await this.getImageModels()
      return mergeModelLists(chatModels, imageModels)
    } catch {
      return chatModels
    }
  },

  async listConversations(page = 1, pageSize = 50): Promise<PaginatedResponse<AIConversation>> {
    const res = await apiClient.get('/user/chat/conversations', {
      params: { page, page_size: pageSize }
    })
    const data = res.data as PaginatedResponse<RawRecord>
    return {
      ...data,
      items: Array.isArray(data.items) ? data.items.map((item) => normalizeConversation(item)) : []
    }
  },

  async createConversation(payload: CreateConversationPayload): Promise<AIConversation> {
    const res = await apiClient.post('/user/chat/conversations', payload)
    return normalizeConversation(res.data as RawRecord)
  },

  async deleteConversation(id: number): Promise<void> {
    await apiClient.delete(`/user/chat/conversations/${id}`)
  },

  async uploadImage(file: File): Promise<UploadImageResult> {
    const formData = new FormData()
    formData.append('file', file)
    const res = await apiClient.post('/user/files/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      timeout: 60000
    })
    return res.data as UploadImageResult
  },

  async generateImages(
    payload: GenerateImagePayload,
    options: GenerateImageOptions = {}
  ): Promise<AIImageGenerationResponse> {
    const res = await apiClient.post('/user/images/generations', payload, {
      timeout: 180000,
      signal: options.signal
    })
    return normalizeImageGenerationResponse((res.data ?? {}) as RawRecord)
  },

  async editImages(
    payload: EditImagePayload,
    options: GenerateImageOptions = {}
  ): Promise<AIImageGenerationResponse> {
    const res = await apiClient.post('/user/images/edits', payload, {
      timeout: 180000,
      signal: options.signal
    })
    return normalizeImageGenerationResponse((res.data ?? {}) as RawRecord)
  },

  async listImageHistory(page = 1, pageSize = 20): Promise<PaginatedResponse<AIImageHistoryItem>> {
    const res = await apiClient.get('/user/image/history', {
      params: { page, page_size: pageSize }
    })
    const data = res.data as PaginatedResponse<RawRecord>
    return {
      ...data,
      items: Array.isArray(data.items) ? data.items.map((item) => normalizeImageHistory(item)) : []
    }
  },

  async streamChatCompletions(
    payload: StreamChatCompletionPayload,
    options: StreamChatCompletionOptions = {}
  ): Promise<string> {
    const response = await fetch(`${API_BASE_URL}/user/chat/completions`, {
      method: 'POST',
      credentials: 'include',
      headers: authHeaders(),
      body: JSON.stringify({ ...payload, stream: true }),
      signal: options.signal
    })

    if (!response.ok) {
      throw new Error(await readErrorMessage(response))
    }

    if (!response.body) {
      const data = await response.json()
      const content = extractAssistantContent(data)
      if (content) options.onDelta?.(content)
      return content
    }

    const contentType = response.headers.get('content-type') || ''
    if (!contentType.includes('text/event-stream')) {
      const text = await response.text()
      try {
        const content = extractAssistantContent(JSON.parse(text))
        if (content) options.onDelta?.(content)
        return content
      } catch {
        if (text) options.onDelta?.(text)
        return text
      }
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let fullText = ''
    let sawDone = false

    while (!sawDone) {
      const { value, done } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const frames = buffer.split(/\r?\n\r?\n/)
      buffer = frames.pop() || ''
      for (const frame of frames) {
        const parsed = parseSSEFrame(frame, options.onDelta)
        fullText += parsed.content
        if (parsed.done) {
          sawDone = true
          break
        }
      }
    }

    if (!sawDone && buffer.trim()) {
      const parsed = parseSSEFrame(buffer, options.onDelta)
      fullText += parsed.content
      sawDone = parsed.done
    }

    return fullText
  }
}
