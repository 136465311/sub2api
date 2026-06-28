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

interface TitleModelSelection {
  model: string
  groupId: number
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
  const trimmed = title.trim()
  return trimmed === '' || trimmed === t('aiChat.untitled') || trimmed === '新会话' || trimmed.toLowerCase() === 'new chat'
}

function normalizeGeneratedTitle(rawTitle: string, source: string): string {
  const sourceNormalized = source.replace(/\s+/g, ' ').trim().toLowerCase()
  let title = rawTitle
    .split(/\r?\n/)[0]
    .replace(/^["'“”‘’「」『』]+|["'“”‘’「」『』]+$/g, '')
    .replace(/[。.!！?？]+$/g, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (!title) return ''
  if (title.toLowerCase() === sourceNormalized) return ''
  const runes = Array.from(title)
  if (runes.length > 32) {
    title = runes.slice(0, 32).join('').trim()
  }
  return title
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
      (conversation.messages || []).length === 0 &&
      isUntitledConversationTitle(conversation.title || '')
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
        void generateTitleForConversation(snapshot)
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

  async function generateTitleForConversation(snapshot: ChatTaskSnapshot): Promise<void> {
    const conversation = conversations.value.find((item) => item.id === snapshot.conversationId)
    if (!conversation || conversation.title_generated || !isUntitledConversationTitle(conversation.title || '')) return
    const titleSource = snapshot.titleSource.trim()
    if (!titleSource) return
    const selection = titleModelSelection(snapshot.groupId, snapshot.model)
    if (!selection) return

    try {
      const rawTitle = await userAiAPI.completeChatCompletions({
        model: selection.model,
        group_id: selection.groupId,
        user_ai_ephemeral: true,
        temperature: 0.2,
        max_tokens: 48,
        messages: [
          {
            role: 'system',
            content: '你是一个会话标题生成器。请根据用户的第一条消息，生成一个简短、自然、准确的会话标题。不要直接复制用户原话，不要加引号，不要加句号，不要解释。标题应概括用户真正想做的事情。中文输入输出中文标题，英文输入输出英文标题。标题长度尽量控制在 6 到 15 个中文字符，或 3 到 8 个英文单词。'
          },
          {
            role: 'user',
            content: `请为下面这条用户消息生成一个会话标题：\n\n${titleSource}`
          }
        ]
      })
      const title = normalizeGeneratedTitle(rawTitle, titleSource)
      if (!title) return
      const updated = await userAiAPI.updateConversationTitle(snapshot.conversationId, { title })
      replaceConversation(updated)
    } catch {
      // Title generation is best-effort and should never affect the main chat flow.
    }
  }

  function titleModelSelection(groupId: number, model: string): TitleModelSelection | null {
    if (!isImageModel(model)) {
      return { model, groupId }
    }
    const sameGroup = chatModelsResult.value.groups.find((group) => group.id === groupId)
    const sameGroupModel = sameGroup?.models.find((item) => !isImageModel(item))
    if (sameGroupModel) {
      return { model: sameGroupModel, groupId }
    }
    for (const group of chatModelsResult.value.groups) {
      const chatModel = group.models.find((item) => !isImageModel(item))
      if (chatModel) {
        return { model: chatModel, groupId: group.id }
      }
    }
    return null
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
