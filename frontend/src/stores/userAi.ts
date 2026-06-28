import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { i18n } from '@/i18n'
import { userAiAPI, type AIChatMessage, type AIConversation, type AIImageGenerationImage, type AIImageHistoryItem, type AIModelGroup, type ChatCompletionMessage, type ChatMessageContent } from '@/api/userAi'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

export type UserAIImageSize = 'auto' | '1:1' | '16:9' | '9:16'

export interface SelectedAIChatImage {
  id: number
  name: string
  type: string
  size: number
  imageUrl: string
}

export interface UserAIDisplayImage {
  url: string
  revisedPrompt: string
}

interface ParsedMessageContent {
  text: string
  imageUrls: string[]
  requestContent: ChatMessageContent
  hasContent: boolean
}

interface FlatImageModelOption {
  value: string
  label: string
  groupId: number
  groupName: string
  model: string
  [key: string]: unknown
}

interface ChatTaskSnapshot {
  taskId: string
  conversationId: number
  assistantMessageId: number
  userContent: string
  titleSource: string
  shouldGenerateTitle: boolean
  model: string
  groupId: number
  images: SelectedAIChatImage[]
}

interface ImageTaskSnapshot {
  taskId: string
  prompt: string
  model: string
  groupId: number
  size: UserAIImageSize
  count: number
}

const maxSelectedImages = 3
let tempMessageId = -1
let taskCounter = 0

function t(key: string, params?: Record<string, unknown>): string {
  return params ? String(i18n.global.t(key, params)) : String(i18n.global.t(key))
}

function nextTaskId(prefix: string): string {
  taskCounter += 1
  return `${prefix}-${Date.now()}-${taskCounter}`
}

function isRequestCanceled(err: unknown): boolean {
  const record = err as { name?: string; code?: string }
  return record?.name === 'AbortError' || record?.name === 'CanceledError' || record?.code === 'ERR_CANCELED'
}

function isImageModel(model: string): boolean {
  const normalized = model.trim().toLowerCase()
  return normalized === 'gpt-image' || normalized.startsWith('gpt-image-') || normalized.startsWith('grok-image')
}

function serializeChatContent(content: ChatMessageContent): string {
  return typeof content === 'string' ? content : JSON.stringify(content)
}

function extractImageUrl(record: Record<string, any>): string {
  const direct = typeof record.url === 'string' ? record.url : ''
  const nested =
    record.image_url && typeof record.image_url === 'object' && typeof record.image_url.url === 'string'
      ? record.image_url.url
      : ''
  const flat = typeof record.image_url === 'string' ? record.image_url : ''
  return String(nested || flat || direct).trim()
}

function isHostedUserAIImageUrl(url: string): boolean {
  return /^\/uploads\/user_ai\/\d+\/(?:generated\/)?[A-Za-z0-9._-]+\.(?:jpg|jpeg|png|webp|gif)$/i.test(url.trim())
}

function isAllowedImageDisplayUrl(url: string): boolean {
  const trimmed = url.trim()
  return isHostedUserAIImageUrl(trimmed) || /^https?:\/\//i.test(trimmed)
}

function parseMessageContent(content: string): ParsedMessageContent {
  const raw = content || ''
  const trimmed = raw.trim()
  if (trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed)) {
        const parts: Exclude<ChatMessageContent, string> = []
        const textParts: string[] = []
        const imageUrls: string[] = []
        for (const item of parsed) {
          if (typeof item === 'string') {
            if (item.trim()) {
              textParts.push(item)
              parts.push({ type: 'text', text: item })
            }
            continue
          }
          if (!item || typeof item !== 'object') continue
          const record = item as Record<string, any>
          const type = String(record.type || '').trim()
          if ((type === 'text' || type === 'input_text' || record.text !== undefined) && typeof record.text === 'string') {
            if (record.text.trim()) {
              textParts.push(record.text)
              parts.push({ type: 'text', text: record.text })
            }
          }

          const imageUrl = extractImageUrl(record)
          if (imageUrl && isAllowedImageDisplayUrl(imageUrl)) {
            imageUrls.push(imageUrl)
            parts.push({
              type: 'image_url',
              image_url: {
                url: imageUrl
              }
            })
          }
        }

        return {
          text: textParts.join('\n'),
          imageUrls,
          requestContent: parts,
          hasContent: textParts.some((part) => part.trim()) || parts.some((part) => part.type === 'image_url')
        }
      }
    } catch {
      // Fall through to plain text rendering for legacy or malformed content.
    }
  }

  return {
    text: raw,
    imageUrls: [],
    requestContent: raw,
    hasContent: Boolean(trimmed)
  }
}

function messageRequestContent(message: AIChatMessage): ChatMessageContent | null {
  const parsed = parseMessageContent(message.content)
  if (!parsed.hasContent) return null
  if (message.role === 'assistant' && parsed.imageUrls.length > 0) {
    return null
  }
  return parsed.requestContent
}

function buildAssistantImageContent(images: AIImageGenerationImage[]): Exclude<ChatMessageContent, string> {
  const content: Exclude<ChatMessageContent, string> = []
  for (const image of images) {
    const imageUrl = String(image.url || (image.b64_json ? `data:image/png;base64,${image.b64_json}` : '')).trim()
    if (!imageUrl) continue
    content.push({
      type: 'image_url',
      image_url: {
        url: imageUrl
      }
    })
    const revisedPrompt = String(image.revised_prompt || '').trim()
    if (revisedPrompt) {
      content.push({ type: 'text', text: revisedPrompt })
    }
  }
  return content
}

function buildUserContent(text: string, images: SelectedAIChatImage[], referenceImageUrls: string[] = []): ChatMessageContent {
  if (images.length === 0 && referenceImageUrls.length === 0) {
    return text
  }

  const content: ChatMessageContent = []
  if (text) {
    content.push({ type: 'text', text })
  }
  for (const imageUrl of referenceImageUrls) {
    content.push({
      type: 'image_url',
      image_url: {
        url: imageUrl
      }
    })
  }
  for (const image of images) {
    content.push({
      type: 'image_url',
      image_url: {
        url: image.imageUrl
      }
    })
  }
  return content
}

function isUntitledConversationTitle(title: string): boolean {
  const trimmed = title.trim().toLowerCase()
  return trimmed === '' || trimmed === t('aiChat.untitled').toLowerCase() || trimmed === '\u65b0\u4f1a\u8bdd' || trimmed === 'new chat' || trimmed === 'new conversation'
}

function isCJKText(text: string): boolean {
  return /[\u3400-\u9fff]/.test(text)
}

function normalizeTitleText(text: string): string {
  return text
    .replace(/[\r\n\t]+/g, ' ')
    .replace(/[“”"''`]+/g, '')
    .replace(/[。.!！?？,，;；:：、()[\]{}<>《》【】]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function buildSimpleConversationTitle(source: string): string {
  const normalized = normalizeTitleText(source)
  if (!normalized) return ''
  return isCJKText(normalized)
    ? buildSimpleChineseTitle(normalized)
    : buildSimpleEnglishTitle(normalized)
}

function buildSimpleChineseTitle(source: string): string {
  const compact = source
    .replace(/\s+/g, '')
    .replace(/^(我有一台|有一台|我有一个|有一个|我有个|有个|一台)/, '')
    .replace(/^(请问|请帮我|帮我|麻烦|能不能|可以|我想要|我想|我需要|想要|如何|怎么|怎样|有没有)/, '')
    .replace(/(一下|一个|一份|这个|那个|吗|呢|吧|的方案|的办法)$/g, '')

  const serverPlaceMatch = compact.match(/([\u3400-\u9fff]{1,4}(?:服务器|主机|节点|VPS))/i)
  const topicPatterns = [
    serverPlaceMatch?.[1] || '',
    matchFirst(compact, /(中转站|中转服务|订阅转换|反向代理|代理服务|服务器|主机|节点|VPS|域名|证书|Docker|数据库|登录|支付|账单|生图|图片|会话|标题|聊天|模型|接口|API)/i),
    matchFirst(compact, /(部署|搭建|配置|优化|修复|排查|生成|改写|翻译|总结|分析|设计|开发|接入|迁移|更新|保存)/i)
  ].filter(Boolean)

  const combined = dedupeStrings(topicPatterns).join('')
  const candidate = combined || compact
  return trimTitleCandidate(candidate, 15)
}

function buildSimpleEnglishTitle(source: string): string {
  const stopWords = new Set(['a', 'an', 'the', 'to', 'for', 'of', 'in', 'on', 'and', 'or', 'with', 'please', 'help', 'me', 'i', 'want', 'need', 'how', 'can', 'could', 'would', 'you'])
  const words = source
    .replace(/[^A-Za-z0-9#+.\s-]/g, ' ')
    .split(/\s+/)
    .map((word) => word.trim())
    .filter((word) => word && !stopWords.has(word.toLowerCase()))
    .slice(0, 8)

  const candidate = words.length > 0 ? words.join(' ') : source.split(/\s+/).slice(0, 6).join(' ')
  return trimTitleCandidate(candidate.replace(/\b\w/g, (char) => char.toUpperCase()), 48)
}

function matchFirst(input: string, pattern: RegExp): string {
  return input.match(pattern)?.[1] || ''
}

function dedupeStrings(items: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const item of items) {
    const key = item.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    result.push(item)
  }
  return result
}

function trimTitleCandidate(candidate: string, maxLength: number): string {
  const title = normalizeTitleText(candidate)
  if (!title) return ''
  const chars = Array.from(title)
  return chars.length > maxLength ? chars.slice(0, maxLength).join('').trim() : title
}

function isRawFirstMessageTitle(title: string, firstMessageText: string): boolean {
  const normalizedTitle = normalizeTitleText(title).toLowerCase()
  const normalizedSource = normalizeTitleText(firstMessageText).toLowerCase()
  if (!normalizedTitle || !normalizedSource) return false
  return normalizedTitle === normalizedSource || (normalizedSource.startsWith(normalizedTitle) && normalizedTitle.length >= 18)
}

function imageToDisplay(image: AIImageGenerationImage): UserAIDisplayImage | null {
  const url = image.url || (image.b64_json ? `data:image/png;base64,${image.b64_json}` : '')
  if (!url) return null
  return {
    url,
    revisedPrompt: image.revised_prompt || ''
  }
}

export const useUserAIStore = defineStore('userAi', () => {
  const appStore = useAppStore()
  const titleUpdateInFlight = new Set<number>()

  const chatModelsResult = ref<{ groups: AIModelGroup[]; default_group_id?: number | null; default_model?: string }>({ groups: [] })
  const conversations = ref<AIConversation[]>([])
  const activeConversationId = ref<number | null>(null)
  const selectedGroupId = ref<number | null>(null)
  const selectedModel = ref('')
  const loadingModels = ref(false)
  const loadingConversations = ref(false)
  const sending = ref(false)
  const imageContinuationHint = ref('')
  const chatLoaded = ref(false)
  const activeChatTaskId = ref<string | null>(null)
  let chatAbortController: AbortController | null = null

  const imagePrompt = ref('')
  const imageSelectedModelKey = ref<string | null>(null)
  const imageSelectedSize = ref<UserAIImageSize>('auto')
  const imageSelectedCount = ref<number>(1)
  const imageGroups = ref<AIModelGroup[]>([])
  const imageDefaultGroupId = ref<number | null>(null)
  const imageDefaultModel = ref('')
  const imageLoadingModels = ref(false)
  const imageGenerating = ref(false)
  const latestImages = ref<UserAIDisplayImage[]>([])
  const latestCreatedAt = ref('')
  const imageHistory = ref<AIImageHistoryItem[]>([])
  const imageHistoryPage = ref(1)
  const imageHistoryTotal = ref(0)
  const imageHistoryLoading = ref(false)
  const imageHistoryLoadingMore = ref(false)
  const imageError = ref('')
  const activeImageTaskId = ref<string | null>(null)

  const selectedGroup = computed(() => {
    return chatModelsResult.value.groups.find((group) => group.id === selectedGroupId.value) || null
  })

  const groupOptions = computed(() =>
    chatModelsResult.value.groups.map((group) => ({
      value: group.id,
      label: group.name,
      platform: group.platform
    }))
  )

  const modelOptions = computed(() =>
    (selectedGroup.value?.models || []).map((model) => ({
      value: model,
      label: model
    }))
  )

  const activeConversation = computed(() => {
    return conversations.value.find((item) => item.id === activeConversationId.value) || null
  })

  const activeMessages = computed(() => activeConversation.value?.messages || [])
  const canCreateConversation = computed(() => Boolean(selectedModel.value && selectedGroupId.value && !sending.value))
  const isSelectedImageModel = computed(() => isImageModel(selectedModel.value))

  const imageModelOptions = computed<FlatImageModelOption[]>(() =>
    imageGroups.value.flatMap((group) =>
      (group.models || []).map((model) => ({
        value: `${group.id}:${model}`,
        label: `${model} / ${group.name}`,
        groupId: group.id,
        groupName: group.name,
        model
      }))
    )
  )

  const imageSelectedModelOption = computed(() =>
    imageModelOptions.value.find((item) => item.value === imageSelectedModelKey.value) || null
  )

  const imageSelectedModelLabel = computed(() => imageSelectedModelOption.value?.label || '')

  const imageSelectedCountNumber = computed(() => {
    const value = Number(imageSelectedCount.value)
    return Number.isFinite(value) && value > 0 ? Math.min(4, Math.floor(value)) : 1
  })

  const canGenerateImage = computed(() =>
    Boolean(imagePrompt.value.trim() && imageSelectedModelOption.value && !imageGenerating.value)
  )

  const hasMoreImageHistory = computed(() => imageHistory.value.length < imageHistoryTotal.value)

  watch(selectedGroupId, () => {
    const models = selectedGroup.value?.models || []
    if (!models.includes(selectedModel.value)) {
      selectedModel.value = models[0] || ''
    }
  })

  watch(activeConversationId, () => {
    syncActiveConversationSelection()
  })

  watch(imageModelOptions, () => {
    applyImageModelSelection()
  })

  async function initializeChat(): Promise<void> {
    const tasks: Array<Promise<void>> = [loadChatModels()]
    if (!sending.value || conversations.value.length === 0) {
      tasks.push(loadConversations(activeConversationId.value, { preserveLocal: sending.value }))
    }
    await Promise.all(tasks)
  }

  async function loadChatModels(): Promise<void> {
    loadingModels.value = true
    try {
      const result = await userAiAPI.getChatModelsWithImages()
      chatModelsResult.value = {
        groups: result.groups || [],
        default_group_id: result.default_group_id,
        default_model: result.default_model
      }

      const defaultGroup = result.default_group_id ?? result.groups?.[0]?.id ?? null
      applyAvailableModelSelection(activeConversation.value, defaultGroup, result.default_model || '')
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('aiChat.loadFailed')))
    } finally {
      loadingModels.value = false
    }
  }

  async function loadConversations(preferredId?: number | null, options: { preserveLocal?: boolean } = {}): Promise<void> {
    loadingConversations.value = true
    try {
      const result = await userAiAPI.listConversations(1, 50)
      conversations.value = mergeConversations(result.items || [], Boolean(options.preserveLocal))
      const nextActiveId =
        preferredId && conversations.value.some((item) => item.id === preferredId)
          ? preferredId
          : conversations.value[0]?.id ?? null
      activeConversationId.value = nextActiveId
      syncActiveConversationSelection()
      chatLoaded.value = true
      void repairMissingConversationTitles(conversations.value)
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('aiChat.loadFailed')))
    } finally {
      loadingConversations.value = false
    }
  }

  function mergeConversations(serverItems: AIConversation[], preserveLocal: boolean): AIConversation[] {
    if (!preserveLocal) return serverItems
    const localById = new Map(conversations.value.map((conversation) => [conversation.id, conversation]))
    const seen = new Set<number>()
    const merged = serverItems.map((serverConversation) => {
      seen.add(serverConversation.id)
      const localConversation = localById.get(serverConversation.id)
      if (!localConversation) return serverConversation
      const localMessages = localConversation.messages.filter((message) => message.id < 0 || message.status)
      if (localMessages.length === 0) return serverConversation
      return {
        ...serverConversation,
        messages: [...serverConversation.messages, ...localMessages]
      }
    })
    const localOnly = conversations.value.filter((conversation) => !seen.has(conversation.id) && conversation.messages.some((message) => message.id < 0 || message.status))
    return [...localOnly, ...merged]
  }

  async function startNewConversation(): Promise<AIConversation | null> {
    if (!canCreateConversation.value) return null
    const conversation = await createConversation(t('aiChat.untitled'))
    if (!conversation) return null
    conversations.value = [conversation, ...conversations.value.filter((item) => item.id !== conversation.id)]
    activeConversationId.value = conversation.id
    return conversation
  }

  async function createConversation(title: string): Promise<AIConversation | null> {
    if (!selectedGroupId.value || !selectedModel.value) return null
    try {
      return await userAiAPI.createConversation({
        title,
        model: selectedModel.value,
        group_id: selectedGroupId.value
      })
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('aiChat.loadFailed')))
      return null
    }
  }

  function selectConversation(id: number): boolean {
    if (sending.value) return false
    activeConversationId.value = id
    syncActiveConversationSelection()
    return true
  }

  async function deleteConversation(id: number): Promise<void> {
    if (id === activeConversationId.value) {
      stopStreaming()
    }
    await userAiAPI.deleteConversation(id)
    conversations.value = conversations.value.filter((item) => item.id !== id)
    if (activeConversationId.value === id) {
      activeConversationId.value = conversations.value[0]?.id ?? null
      syncActiveConversationSelection()
    }
  }

  async function sendMessage(textInput: string, selectedImages: SelectedAIChatImage[]): Promise<boolean> {
    const text = textInput.trim()
    const images = selectedImages.map((image) => ({ ...image }))
    if ((!text && images.length === 0) || !selectedModel.value || !selectedGroupId.value || sending.value) return false
    if (isImageModel(selectedModel.value) && !text) return false

    const existingConversation = activeConversation.value
    const conversation = existingConversation || (await createConversation(t('aiChat.untitled')))
    if (!conversation) return false

    if (!existingConversation) {
      conversations.value = [conversation, ...conversations.value]
      activeConversationId.value = conversation.id
    }

    const taskId = nextTaskId('chat')
    const model = selectedModel.value
    const groupId = selectedGroupId.value
    const historyReferenceImageUrls =
      isImageModel(model) && images.length === 0
        ? latestImageModelReferenceUrls(conversation.id)
        : []
    if (historyReferenceImageUrls.length > 0) {
      imageContinuationHint.value = t('aiChat.continuingImageEdit')
      appStore.showInfo(t('aiChat.continuingImageEdit'))
    }
    const userContent = serializeChatContent(buildUserContent(text, images, historyReferenceImageUrls))
    const shouldGenerateTitle =
      !conversation.title_generated &&
      ((conversation.messages || []).length === 0 || isRawFirstMessageTitle(conversation.title || '', text)) &&
      (isUntitledConversationTitle(conversation.title || '') || isRawFirstMessageTitle(conversation.title || '', text))
    const userMessage = makeLocalMessage(conversation.id, 'user', userContent, taskId, model)
    const assistantMessage = makeLocalMessage(conversation.id, 'assistant', '', taskId, model)
    assistantMessage.status = 'pending'
    appendLocalMessages(conversation.id, [userMessage, assistantMessage])

    sending.value = true
    activeChatTaskId.value = taskId
    chatAbortController = new AbortController()

    const snapshot: ChatTaskSnapshot = {
      taskId,
      conversationId: conversation.id,
      assistantMessageId: assistantMessage.id,
      userContent,
      titleSource: text || parseMessageContent(userContent).text || t('aiChat.imageConversationTitle'),
      shouldGenerateTitle,
      model,
      groupId,
      images
    }

    void runChatTask(snapshot, chatAbortController.signal)
    return true
  }

  async function runChatTask(snapshot: ChatTaskSnapshot, signal: AbortSignal): Promise<void> {
    try {
      if (isImageModel(snapshot.model)) {
        const imagePayload = {
          prompt: snapshot.titleSource,
          model: snapshot.model,
          group_id: snapshot.groupId,
          conversation_id: snapshot.conversationId
        }
        const result = snapshot.images.length > 0
          ? await userAiAPI.editImages(
            {
              ...imagePayload,
              image_urls: snapshot.images.map((image) => image.imageUrl)
            },
            { signal }
          )
          : await userAiAPI.generateImages(imagePayload, { signal })
        const assistantImageContent = buildAssistantImageContent(result.data)
        if (assistantImageContent.length === 0) {
          throw new Error(t('aiImage.generateFailed'))
        }
        updateLocalMessageContent(snapshot.conversationId, snapshot.assistantMessageId, serializeChatContent(assistantImageContent))
      } else {
        const userMessage = findLocalMessage(snapshot.conversationId, -1, snapshot.taskId, 'user')
        if (!userMessage) {
          throw new Error(t('aiChat.sendFailed'))
        }
        const requestMessages = buildRequestMessages(snapshot.conversationId, userMessage)
        const streamedContent = await userAiAPI.streamChatCompletions(
          {
            model: snapshot.model,
            group_id: snapshot.groupId,
            conversation_id: snapshot.conversationId,
            messages: requestMessages,
            stream: true
          },
          {
            signal,
            onDelta: (delta) => {
              appendAssistantDelta(snapshot.conversationId, snapshot.assistantMessageId, delta)
            }
          }
        )
        if (!getLocalMessageContent(snapshot.conversationId, snapshot.assistantMessageId) && streamedContent) {
          updateLocalMessageContent(snapshot.conversationId, snapshot.assistantMessageId, streamedContent)
        }
      }

      clearLocalMessageStatus(snapshot.conversationId, snapshot.assistantMessageId)
      await loadConversations(snapshot.conversationId, { preserveLocal: false })
      if (snapshot.shouldGenerateTitle) {
        void saveSimpleTitleForConversation(snapshot)
      }
    } catch (err) {
      if (isRequestCanceled(err)) {
        clearLocalMessageStatus(snapshot.conversationId, snapshot.assistantMessageId)
        return
      }
      await loadConversations(snapshot.conversationId, { preserveLocal: true })
      if (hasSavedAssistantReply(snapshot.conversationId, snapshot.userContent)) {
        await loadConversations(snapshot.conversationId, { preserveLocal: false })
        return
      }
      const message = extractApiErrorMessage(err, t('aiChat.sendFailed'))
      markLocalMessageFailed(snapshot.conversationId, snapshot.assistantMessageId, message)
      appStore.showError(message)
    } finally {
      if (activeChatTaskId.value === snapshot.taskId) {
        sending.value = false
        activeChatTaskId.value = null
        chatAbortController = null
        imageContinuationHint.value = ''
      }
    }
  }

  async function saveSimpleTitleForConversation(snapshot: ChatTaskSnapshot): Promise<void> {
    const conversation = conversations.value.find((item) => item.id === snapshot.conversationId)
    if (!conversation) return
    await saveSimpleTitle(conversation, snapshot.titleSource)
  }

  async function repairMissingConversationTitles(items: AIConversation[]): Promise<void> {
    const candidates = items.filter((conversation) => !conversation.title_generated).slice(0, 10)
    await Promise.all(candidates.map(async (conversation) => {
      const firstUserMessage = (conversation.messages || []).find((message) => message.id > 0 && message.role === 'user')
      if (!firstUserMessage) return
      const titleSource = parseMessageContent(firstUserMessage.content).text || t('aiChat.imageConversationTitle')
      await saveSimpleTitle(conversation, titleSource)
    }))
  }

  async function saveSimpleTitle(conversation: AIConversation, titleSourceInput: string): Promise<void> {
    if (conversation.title_generated || titleUpdateInFlight.has(conversation.id)) return
    const titleSource = titleSourceInput.trim()
    if (!titleSource) return
    if (!isUntitledConversationTitle(conversation.title || '') && !isRawFirstMessageTitle(conversation.title || '', titleSource)) return

    const title = buildSimpleConversationTitle(titleSource)
    if (!title) return

    try {
      titleUpdateInFlight.add(conversation.id)
      const updated = await userAiAPI.updateConversationTitle(conversation.id, { title })
      replaceConversation(updated)
    } catch {
      // Title saving is best-effort and should never affect the main chat flow.
    } finally {
      titleUpdateInFlight.delete(conversation.id)
    }
  }

  function makeLocalMessage(conversationId: number, role: string, content: string, taskId: string, model: string): AIChatMessage {
    return {
      id: tempMessageId--,
      conversation_id: conversationId,
      user_id: 0,
      role,
      content,
      model,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      task_id: taskId
    }
  }

  function appendLocalMessages(conversationId: number, messages: AIChatMessage[]): void {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    if (!conversation) return
    conversation.messages = [...conversation.messages, ...messages]
    conversation.updated_at = new Date().toISOString()
  }

  function findLocalMessage(conversationId: number, messageId: number, taskId?: string, role?: string): AIChatMessage | null {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    const messages = conversation?.messages || []
    if (messageId >= 0) {
      return messages.find((item) => item.id === messageId) || null
    }
    if (!taskId) {
      return messages.find((item) => item.id === messageId) || null
    }
    return messages.find((item) => item.task_id === taskId && (!role || item.role === role)) || null
  }

  function getLocalMessageContent(conversationId: number, messageId: number): string {
    return findLocalMessage(conversationId, messageId)?.content || ''
  }

  function updateLocalMessageContent(conversationId: number, messageId: number, content: string): void {
    const message = findLocalMessage(conversationId, messageId)
    if (message) {
      message.content = content
      message.status = undefined
      message.error = undefined
    }
  }

  function clearLocalMessageStatus(conversationId: number, messageId: number): void {
    const message = findLocalMessage(conversationId, messageId)
    if (!message) return
    message.status = undefined
    message.error = undefined
  }

  function markLocalMessageFailed(conversationId: number, messageId: number, error: string): void {
    const message = findLocalMessage(conversationId, messageId)
    if (!message) return
    message.status = 'failed'
    message.error = error
    message.content = error
  }

  function appendAssistantDelta(conversationId: number, messageId: number, delta: string): void {
    const message = findLocalMessage(conversationId, messageId)
    if (message) {
      message.content = `${message.content || ''}${delta}`
      message.status = undefined
      message.error = undefined
    }
  }

  function hasSavedAssistantReply(conversationId: number, userContent: string): boolean {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    const messages = conversation?.messages || []
    for (let i = messages.length - 1; i >= 0; i--) {
      const message = messages[i]
      if (message.role !== 'user' || message.content.trim() !== userContent.trim() || message.id < 0) continue
      return messages.slice(i + 1).some((item) => item.role === 'assistant' && item.id > 0 && Boolean(item.content.trim()))
    }
    return false
  }

  function buildRequestMessages(conversationId: number, userMessage: AIChatMessage): ChatCompletionMessage[] {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    return (conversation?.messages || [])
      .filter((message) => message.id > 0 || message.id === userMessage.id)
      .filter((message) => {
        if (!['system', 'user', 'assistant'].includes(message.role)) return false
        return messageRequestContent(message) !== null
      })
      .map((message) => ({
        role: message.role as 'system' | 'user' | 'assistant',
        content: messageRequestContent(message) as ChatMessageContent
      }))
  }

  function latestImageModelReferenceUrls(conversationId: number): string[] {
    const conversation = conversations.value.find((item) => item.id === conversationId)
    const messages = conversation?.messages || []
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role !== 'assistant') continue
      const imageUrls = parseMessageContent(messages[i].content).imageUrls.filter(isHostedUserAIImageUrl)
      if (imageUrls.length > 0) return imageUrls.slice(0, maxSelectedImages)
    }
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role !== 'user') continue
      const imageUrls = parseMessageContent(messages[i].content).imageUrls.filter(isHostedUserAIImageUrl)
      if (imageUrls.length > 0) return imageUrls.slice(0, maxSelectedImages)
    }
    return []
  }

  function syncActiveConversationSelection(): void {
    applyAvailableModelSelection(activeConversation.value, chatModelsResult.value.default_group_id ?? chatModelsResult.value.groups[0]?.id ?? null, chatModelsResult.value.default_model || '')
  }

  function applyAvailableModelSelection(
    conversation: AIConversation | null,
    defaultGroupId: number | null,
    defaultModel: string
  ): void {
    const groups = chatModelsResult.value.groups || []
    if (groups.length === 0) {
      selectedGroupId.value = null
      selectedModel.value = ''
      return
    }

    const conversationGroup = conversation?.group_id
    const preferredGroupId =
      conversationGroup && groups.some((group) => group.id === conversationGroup)
        ? conversationGroup
        : selectedGroupId.value && groups.some((group) => group.id === selectedGroupId.value)
          ? selectedGroupId.value
          : defaultGroupId && groups.some((group) => group.id === defaultGroupId)
            ? defaultGroupId
            : groups[0]?.id ?? null

    selectedGroupId.value = preferredGroupId

    const models = groups.find((group) => group.id === preferredGroupId)?.models || []
    if (models.length === 0) {
      selectedModel.value = ''
      return
    }

    const conversationModel = conversation?.model || ''
    if (conversationModel && models.includes(conversationModel)) {
      selectedModel.value = conversationModel
      return
    }
    if (selectedModel.value && models.includes(selectedModel.value)) {
      return
    }
    if (defaultModel && models.includes(defaultModel)) {
      selectedModel.value = defaultModel
      return
    }
    selectedModel.value = models[0] || ''
  }

  function replaceConversation(updated: AIConversation): void {
    const index = conversations.value.findIndex((item) => item.id === updated.id)
    if (index >= 0) {
      conversations.value[index] = updated
      return
    }
    conversations.value = [updated, ...conversations.value]
  }

  function stopStreaming(): void {
    chatAbortController?.abort()
    chatAbortController = null
    sending.value = false
    activeChatTaskId.value = null
  }

  async function initializeImage(): Promise<void> {
    await Promise.all([loadImageModels(), loadImageHistory(true)])
  }

  async function loadImageModels(): Promise<void> {
    imageLoadingModels.value = true
    try {
      const result = await userAiAPI.getImageModels()
      imageGroups.value = result.groups || []
      imageDefaultGroupId.value = result.default_group_id ?? null
      imageDefaultModel.value = result.default_model || ''
      applyImageModelSelection()
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('aiImage.loadFailed')))
    } finally {
      imageLoadingModels.value = false
    }
  }

  function applyImageModelSelection(): void {
    const options = imageModelOptions.value
    if (options.length === 0) {
      imageSelectedModelKey.value = null
      return
    }
    if (imageSelectedModelKey.value && options.some((item) => item.value === imageSelectedModelKey.value)) {
      return
    }
    const preferred = options.find((item) => item.groupId === imageDefaultGroupId.value && item.model === imageDefaultModel.value)
    imageSelectedModelKey.value = (preferred || options[0]).value
  }

  async function loadImageHistory(reset: boolean): Promise<void> {
    if (reset) {
      imageHistoryLoading.value = true
      imageHistoryPage.value = 1
    } else {
      imageHistoryLoadingMore.value = true
    }

    try {
      const page = reset ? 1 : imageHistoryPage.value + 1
      const result = await userAiAPI.listImageHistory(page, 20)
      imageHistory.value = reset ? result.items : [...imageHistory.value, ...result.items]
      imageHistoryPage.value = result.page
      imageHistoryTotal.value = result.total
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('aiImage.loadFailed')))
    } finally {
      imageHistoryLoading.value = false
      imageHistoryLoadingMore.value = false
    }
  }

  function generateImages(): boolean {
    if (!canGenerateImage.value || !imageSelectedModelOption.value) return false
    const selection = imageSelectedModelOption.value
    const taskId = nextTaskId('image')
    const snapshot: ImageTaskSnapshot = {
      taskId,
      prompt: imagePrompt.value.trim(),
      model: selection.model,
      groupId: selection.groupId,
      size: imageSelectedSize.value,
      count: imageSelectedCountNumber.value
    }
    imageGenerating.value = true
    activeImageTaskId.value = taskId
    imageError.value = ''
    void runImageTask(snapshot)
    return true
  }

  async function runImageTask(snapshot: ImageTaskSnapshot): Promise<void> {
    try {
      const result = await userAiAPI.generateImages({
        prompt: snapshot.prompt,
        model: snapshot.model,
        group_id: snapshot.groupId,
        size: snapshot.size,
        n: snapshot.count
      })

      latestImages.value = result.data.map(imageToDisplay).filter((item): item is UserAIDisplayImage => Boolean(item))
      latestCreatedAt.value = result.created > 0 ? new Date(result.created * 1000).toISOString() : new Date().toISOString()
      appStore.showSuccess(t('aiImage.generateSuccess'))
      void loadImageHistory(true)
    } catch (err) {
      imageError.value = extractApiErrorMessage(err, t('aiImage.generateFailed'))
      appStore.showError(imageError.value)
    } finally {
      if (activeImageTaskId.value === snapshot.taskId) {
        imageGenerating.value = false
        activeImageTaskId.value = null
      }
    }
  }

  return {
    chatModelsResult,
    conversations,
    activeConversationId,
    selectedGroupId,
    selectedModel,
    loadingModels,
    loadingConversations,
    sending,
    imageContinuationHint,
    chatLoaded,
    activeChatTaskId,
    selectedGroup,
    groupOptions,
    modelOptions,
    activeConversation,
    activeMessages,
    canCreateConversation,
    isSelectedImageModel,
    initializeChat,
    loadChatModels,
    loadConversations,
    startNewConversation,
    createConversation,
    selectConversation,
    deleteConversation,
    sendMessage,
    stopStreaming,
    imagePrompt,
    imageSelectedModelKey,
    imageSelectedSize,
    imageSelectedCount,
    imageGroups,
    imageLoadingModels,
    imageGenerating,
    latestImages,
    latestCreatedAt,
    imageHistory,
    imageHistoryPage,
    imageHistoryTotal,
    imageHistoryLoading,
    imageHistoryLoadingMore,
    imageError,
    imageModelOptions,
    imageSelectedModelOption,
    imageSelectedModelLabel,
    imageSelectedCountNumber,
    canGenerateImage,
    hasMoreImageHistory,
    initializeImage,
    loadImageModels,
    applyImageModelSelection,
    loadImageHistory,
    generateImages
  }
})
