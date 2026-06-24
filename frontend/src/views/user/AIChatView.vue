<template>
  <AppLayout>
    <template #header>
      <div class="flex flex-col gap-1">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('aiChat.title') }}</h1>
        <p class="text-sm text-gray-600 dark:text-dark-400">{{ t('aiChat.description') }}</p>
      </div>
    </template>

    <div class="ai-chat-shell">
      <aside class="conversation-panel">
        <div class="conversation-panel-header">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('aiChat.conversations') }}
            </h2>
            <p class="text-xs text-gray-500 dark:text-dark-400">
              {{ conversations.length }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-ghost btn-sm btn-icon"
              :title="t('common.refresh')"
              :disabled="loadingConversations"
              @click="loadConversations(activeConversationId)"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingConversations }" />
            </button>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!canCreateConversation"
              @click="startNewConversation"
            >
              <Icon name="plus" size="sm" />
              <span>{{ t('aiChat.newConversation') }}</span>
            </button>
          </div>
        </div>

        <div class="conversation-list">
          <button
            v-for="conversation in conversations"
            :key="conversation.id"
            type="button"
            class="conversation-item"
            :class="{ 'conversation-item-active': conversation.id === activeConversationId }"
            @click="selectConversation(conversation.id)"
          >
            <span class="min-w-0 flex-1 text-left">
              <span class="block truncate text-sm font-medium">
                {{ conversationTitle(conversation) }}
              </span>
              <span class="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span class="truncate">{{ conversation.model || t('aiChat.noModel') }}</span>
                <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-500"></span>
                <span class="shrink-0">{{ formatRelativeTime(conversation.updated_at || conversation.created_at) }}</span>
              </span>
            </span>
            <span
              class="conversation-delete"
              role="button"
              tabindex="0"
              :title="t('aiChat.deleteConversation')"
              @click.stop="requestDeleteConversation(conversation.id)"
              @keydown.enter.stop.prevent="requestDeleteConversation(conversation.id)"
            >
              <Icon name="trash" size="sm" />
            </span>
          </button>

          <div v-if="!loadingConversations && conversations.length === 0" class="empty-conversations">
            <Icon name="chat" size="lg" />
            <span>{{ t('aiChat.noConversations') }}</span>
          </div>
        </div>
      </aside>

      <section class="chat-panel">
        <header class="mobile-chat-header">
          <div class="mobile-title-row">
            <div class="min-w-0">
              <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('aiChat.title') }}
              </h2>
              <p class="truncate text-xs text-gray-500 dark:text-dark-400">
                {{ activeConversation ? conversationTitle(activeConversation) : t('aiChat.emptyTitle') }}
              </p>
            </div>
            <button
              type="button"
              class="mobile-conversations-button"
              @click="mobileConversationsOpen = true"
            >
              <Icon name="chat" size="sm" />
              <span>{{ t('aiChat.conversations') }}</span>
            </button>
          </div>

          <div class="mobile-model-controls">
            <label class="mobile-select-pill">
              <span>{{ t('aiChat.modelGroup') }}:</span>
              <Select
                v-model="selectedGroupIdValue"
                :options="groupOptions"
                :placeholder="t('aiChat.selectGroup')"
                :disabled="loadingModels || sending || groupOptions.length === 0"
                :empty-text="t('aiChat.noModels')"
              />
            </label>
            <label class="mobile-select-pill">
              <span>{{ t('aiChat.model') }}:</span>
              <Select
                v-model="selectedModel"
                :options="modelOptions"
                :placeholder="loadingModels ? t('aiChat.loadingModels') : t('aiChat.selectModel')"
                :disabled="loadingModels || sending || modelOptions.length === 0"
                :searchable="true"
                :empty-text="t('aiChat.noModels')"
              />
            </label>
          </div>
        </header>

        <header class="chat-toolbar">
          <div class="min-w-0">
            <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
              {{ activeConversation ? conversationTitle(activeConversation) : t('aiChat.emptyTitle') }}
            </h2>
            <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">
              {{ selectedModel || t('aiChat.noModel') }}
            </p>
          </div>

          <div class="chat-model-controls">
            <label class="control-label">
              <span>{{ t('aiChat.modelGroup') }}</span>
              <Select
                v-model="selectedGroupIdValue"
                :options="groupOptions"
                :placeholder="t('aiChat.selectGroup')"
                :disabled="loadingModels || sending || groupOptions.length === 0"
                :empty-text="t('aiChat.noModels')"
              />
            </label>
            <label class="control-label">
              <span>{{ t('aiChat.model') }}</span>
              <Select
                v-model="selectedModel"
                :options="modelOptions"
                :placeholder="loadingModels ? t('aiChat.loadingModels') : t('aiChat.selectModel')"
                :disabled="loadingModels || sending || modelOptions.length === 0"
                :searchable="true"
                :empty-text="t('aiChat.noModels')"
              />
            </label>
          </div>
        </header>

        <main ref="messagesContainerRef" class="messages-panel">
          <div v-if="activeMessages.length === 0" class="empty-chat">
            <div class="empty-chat-icon">
              <Icon name="brain" size="lg" />
            </div>
            <h3>{{ t('aiChat.emptyTitle') }}</h3>
            <p>{{ t('aiChat.emptyDescription') }}</p>
          </div>

          <div v-else class="message-list">
            <article
              v-for="message in activeMessages"
              :key="message.id"
              class="message-row"
              :class="message.role === 'user' ? 'message-row-user' : 'message-row-assistant'"
            >
              <div class="message-bubble">
                <div class="message-meta">
                  <span>{{ message.role === 'user' ? t('aiChat.you') : t('aiChat.assistant') }}</span>
                  <span v-if="message.model">{{ message.model }}</span>
                </div>
                <template v-if="message.role === 'assistant'">
                  <div v-if="messageImageUrls(message).length" class="assistant-image-list">
                    <figure
                      v-for="(imageUrl, index) in messageImageUrls(message)"
                      :key="`${message.id}-assistant-image-${index}`"
                      class="assistant-image-card"
                    >
                      <button
                        type="button"
                        class="assistant-image-preview"
                        :title="t('common.expand')"
                        :aria-label="t('common.expand')"
                        @click="openImagePreview(imageUrl, t('aiChat.generatedImageAlt', { count: index + 1 }))"
                      >
                        <img
                          :src="imageUrl"
                          :alt="t('aiChat.generatedImageAlt', { count: index + 1 })"
                          loading="lazy"
                        />
                      </button>
                      <button
                        type="button"
                        class="message-download-button"
                        @click="downloadImage(imageUrl, `ai-chat-${Math.abs(message.id)}-${index + 1}`)"
                      >
                        <Icon name="download" size="sm" />
                        <span>{{ t('aiChat.download') }}</span>
                      </button>
                    </figure>
                  </div>
                  <div
                    v-if="messageText(message) || (!messageImageUrls(message).length && sending)"
                    class="markdown-body"
                    v-html="renderMarkdown(messageText(message) || t('aiChat.streaming'))"
                  ></div>
                </template>
                <template v-else>
                  <div v-if="messageImageUrls(message).length" class="message-image-grid">
                    <button
                      v-for="(imageUrl, index) in messageImageUrls(message)"
                      :key="`${message.id}-image-${index}`"
                      type="button"
                      class="message-image-button"
                      :title="t('common.expand')"
                      :aria-label="t('common.expand')"
                      @click="openImagePreview(imageUrl, t('aiChat.imageAlt', { count: index + 1 }))"
                    >
                      <img
                        :src="imageUrl"
                        :alt="t('aiChat.imageAlt', { count: index + 1 })"
                        class="message-image-thumb"
                        loading="lazy"
                      />
                    </button>
                  </div>
                  <p v-if="messageText(message)" class="whitespace-pre-wrap break-words">{{ messageText(message) }}</p>
                </template>
              </div>
            </article>
          </div>
        </main>

        <footer class="composer">
          <div class="composer-editor">
            <div v-if="imageContinuationHint" class="image-continuation-hint">
              <Icon name="sparkles" size="xs" />
              <span>{{ imageContinuationHint }}</span>
            </div>
            <div v-if="selectedImages.length" class="selected-image-strip">
              <div
                v-for="image in selectedImages"
                :key="image.id"
                class="selected-image-item"
              >
                <img :src="image.imageUrl" :alt="image.name" class="selected-image-thumb" />
                <button
                  type="button"
                  class="selected-image-remove"
                  :title="t('aiChat.removeImage')"
                  :aria-label="t('aiChat.removeImage')"
                  :disabled="sending"
                  @click="removeSelectedImage(image.id)"
                >
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </div>
            <div v-if="imageUploading" class="image-upload-progress">
              <Icon name="refresh" size="sm" class="animate-spin" />
              <span>{{ t('aiChat.imageUploading') }}</span>
            </div>
            <div class="composer-input-row">
              <input
                ref="imageInputRef"
                class="sr-only"
                type="file"
                accept="image/jpeg,image/png,image/webp,image/gif"
                multiple
                :disabled="sending || imageUploading || selectedImages.length >= maxSelectedImages"
                @change="handleImageSelection"
              />
              <button
                type="button"
                class="image-upload-button"
                :title="t('aiChat.uploadImage')"
                :aria-label="t('aiChat.uploadImage')"
                :disabled="sending || imageUploading || selectedImages.length >= maxSelectedImages"
                @click="openImagePicker"
              >
                <Icon v-if="imageUploading" name="refresh" size="sm" class="animate-spin" />
                <Icon v-else name="upload" size="sm" />
              </button>
              <textarea
                ref="inputRef"
                v-model="draft"
                class="input composer-input"
                rows="3"
                :placeholder="t('aiChat.inputPlaceholder')"
                :disabled="sending || !selectedModel"
                @keydown.enter.exact.prevent="sendMessage"
              ></textarea>
            </div>
          </div>
          <div class="composer-actions">
            <p class="truncate text-xs text-gray-500 dark:text-dark-400">
              {{ selectedGroup?.name || t('aiChat.selectGroup') }}
            </p>
            <div class="flex items-center gap-2">
              <button
                v-if="sending"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="stopStreaming"
              >
                <Icon name="x" size="sm" />
                <span>{{ t('aiChat.stop') }}</span>
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="!canSend"
                @click="sendMessage"
              >
                <Icon name="arrowUp" size="sm" />
                <span>{{ t('aiChat.send') }}</span>
              </button>
            </div>
          </div>
        </footer>
      </section>
    </div>

    <Teleport to="body">
      <Transition name="mobile-drawer-fade">
        <div
          v-if="mobileConversationsOpen"
          class="mobile-conversation-overlay"
          @click="mobileConversationsOpen = false"
        ></div>
      </Transition>
      <Transition name="mobile-drawer-slide">
        <aside
          v-if="mobileConversationsOpen"
          class="mobile-conversation-drawer"
          role="dialog"
          aria-modal="true"
          :aria-label="t('aiChat.conversations')"
        >
          <div class="mobile-drawer-header">
            <div>
              <h2>{{ t('aiChat.conversations') }}</h2>
              <p>{{ conversations.length }}</p>
            </div>
            <button
              type="button"
              class="mobile-drawer-close"
              :title="t('common.cancel')"
              @click="mobileConversationsOpen = false"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="mobile-drawer-actions">
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              :disabled="loadingConversations"
              @click="loadConversations(activeConversationId)"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingConversations }" />
              <span>{{ t('common.refresh') }}</span>
            </button>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!canCreateConversation"
              @click="startNewConversation"
            >
              <Icon name="plus" size="sm" />
              <span>{{ t('aiChat.newConversation') }}</span>
            </button>
          </div>

          <div class="mobile-drawer-list">
            <button
              v-for="conversation in conversations"
              :key="conversation.id"
              type="button"
              class="conversation-item"
              :class="{ 'conversation-item-active': conversation.id === activeConversationId }"
              @click="selectConversation(conversation.id)"
            >
              <span class="min-w-0 flex-1 text-left">
                <span class="block truncate text-sm font-medium">
                  {{ conversationTitle(conversation) }}
                </span>
                <span class="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span class="truncate">{{ conversation.model || t('aiChat.noModel') }}</span>
                  <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-500"></span>
                  <span class="shrink-0">{{ formatRelativeTime(conversation.updated_at || conversation.created_at) }}</span>
                </span>
              </span>
              <span
                class="conversation-delete"
                role="button"
                tabindex="0"
                :title="t('aiChat.deleteConversation')"
                @click.stop="requestDeleteConversation(conversation.id)"
                @keydown.enter.stop.prevent="requestDeleteConversation(conversation.id)"
              >
                <Icon name="trash" size="sm" />
              </span>
            </button>

            <div v-if="!loadingConversations && conversations.length === 0" class="empty-conversations">
              <Icon name="chat" size="lg" />
              <span>{{ t('aiChat.noConversations') }}</span>
            </div>
          </div>
        </aside>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="mobile-drawer-fade">
        <div
          v-if="previewImageUrl"
          class="image-preview-overlay"
          role="dialog"
          aria-modal="true"
          :aria-label="previewImageAlt || t('aiChat.generatedImageAlt', { count: 1 })"
          @click.self="closeImagePreview"
        >
          <div class="image-preview-dialog">
            <div class="image-preview-toolbar">
              <button
                type="button"
                class="image-preview-toolbar-button"
                :title="t('aiChat.download')"
                :aria-label="t('aiChat.download')"
                @click="downloadImage(previewImageUrl, `ai-chat-preview-${Date.now()}`)"
              >
                <Icon name="download" size="sm" />
              </button>
              <button
                type="button"
                class="image-preview-toolbar-button"
                :title="t('common.close')"
                :aria-label="t('common.close')"
                @click="closeImagePreview"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
            <img :src="previewImageUrl" :alt="previewImageAlt" class="image-preview-full" />
          </div>
        </div>
      </Transition>
    </Teleport>

    <ConfirmDialog
      :show="pendingDeleteId !== null"
      :title="t('aiChat.deleteConversation')"
      :message="t('aiChat.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDeleteConversation"
      @cancel="pendingDeleteId = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  userAiAPI,
  type AIConversation,
  type AIChatMessage,
  type AIModelGroup,
  type ChatCompletionMessage,
  type ChatMessageContent
} from '@/api/userAi'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatRelativeTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const modelsResult = ref<{ groups: AIModelGroup[]; default_group_id?: number | null; default_model?: string }>({ groups: [] })
const conversations = ref<AIConversation[]>([])
const activeConversationId = ref<number | null>(null)
const selectedGroupId = ref<number | null>(null)
const selectedModel = ref<string>('')
const draft = ref('')
const selectedImages = ref<SelectedImage[]>([])
const imageContinuationHint = ref('')
const loadingModels = ref(false)
const loadingConversations = ref(false)
const sending = ref(false)
const imageUploading = ref(false)
const pendingDeleteId = ref<number | null>(null)
const abortController = ref<AbortController | null>(null)
const messagesContainerRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)
const imageInputRef = ref<HTMLInputElement | null>(null)
const mobileConversationsOpen = ref(false)
const previewImageUrl = ref('')
const previewImageAlt = ref('')
let tempMessageId = -1
let selectedImageId = 1

const maxSelectedImages = 3
const maxOriginalImageBytes = 20 * 1024 * 1024
const maxCompressedImageBytes = 2 * 1024 * 1024
const maxImageDimension = 1600
const imageJPEGQuality = 0.82
const allowedImageTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif'])

interface SelectedImage {
  id: number
  name: string
  type: string
  size: number
  imageUrl: string
}

interface ParsedMessageContent {
  text: string
  imageUrls: string[]
  requestContent: ChatMessageContent
  hasContent: boolean
}

marked.use({ gfm: true, breaks: true })

const selectedGroupIdValue = computed({
  get: () => selectedGroupId.value,
  set: (value: string | number | boolean | null) => {
    selectedGroupId.value = typeof value === 'number' ? value : value ? Number(value) : null
  }
})

const selectedGroup = computed(() => {
  return modelsResult.value.groups.find((group) => group.id === selectedGroupId.value) || null
})

const groupOptions = computed(() =>
  modelsResult.value.groups.map((group) => ({
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

const canCreateConversation = computed(() => Boolean(selectedModel.value && selectedGroupId.value))
const isSelectedImageModel = computed(() => isImageModel(selectedModel.value))

const canSend = computed(() => {
  if (isSelectedImageModel.value) {
    return Boolean(
      draft.value.trim() &&
      selectedModel.value &&
      selectedGroupId.value &&
      !imageUploading.value &&
      !sending.value
    )
  }
  return Boolean(
    (draft.value.trim() || selectedImages.value.length > 0) &&
    selectedModel.value &&
    selectedGroupId.value &&
    !imageUploading.value &&
    !sending.value
  )
})

watch(selectedGroupId, () => {
  const models = selectedGroup.value?.models || []
  if (!models.includes(selectedModel.value)) {
    selectedModel.value = models[0] || ''
  }
})

watch(activeConversationId, () => {
  syncActiveConversationSelection()
})

onMounted(async () => {
  await Promise.all([loadModels(), loadConversations()])
})

onBeforeUnmount(() => {
  stopStreaming()
})

async function loadModels(): Promise<void> {
  loadingModels.value = true
  try {
    const result = await userAiAPI.getChatModelsWithImages()
    modelsResult.value = {
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

async function loadConversations(preferredId?: number | null): Promise<void> {
  loadingConversations.value = true
  try {
    const result = await userAiAPI.listConversations(1, 50)
    conversations.value = result.items || []
    const nextActiveId =
      preferredId && conversations.value.some((item) => item.id === preferredId)
        ? preferredId
        : conversations.value[0]?.id ?? null
    activeConversationId.value = nextActiveId
    syncActiveConversationSelection()
    await scrollToBottom()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('aiChat.loadFailed')))
  } finally {
    loadingConversations.value = false
  }
}

async function startNewConversation(): Promise<void> {
  const conversation = await createConversation(t('aiChat.untitled'))
  if (!conversation) return
  conversations.value = [conversation, ...conversations.value.filter((item) => item.id !== conversation.id)]
  activeConversationId.value = conversation.id
  mobileConversationsOpen.value = false
  await nextTick()
  inputRef.value?.focus()
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

function selectConversation(id: number): void {
  if (sending.value) return
  activeConversationId.value = id
  mobileConversationsOpen.value = false
}

function requestDeleteConversation(id: number): void {
  pendingDeleteId.value = id
}

async function confirmDeleteConversation(): Promise<void> {
  if (!pendingDeleteId.value) return
  const deleteId = pendingDeleteId.value
  try {
    if (deleteId === activeConversationId.value) {
      stopStreaming()
    }
    await userAiAPI.deleteConversation(deleteId)
    conversations.value = conversations.value.filter((item) => item.id !== deleteId)
    if (activeConversationId.value === deleteId) {
      activeConversationId.value = conversations.value[0]?.id ?? null
    }
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('aiChat.deleteFailed')))
  } finally {
    pendingDeleteId.value = null
  }
}

async function sendMessage(): Promise<void> {
  const text = draft.value.trim()
  const images = selectedImages.value.map((image) => ({ ...image }))
  if ((!text && images.length === 0) || !selectedModel.value || !selectedGroupId.value || sending.value) return
  if (isImageModel(selectedModel.value) && !text) return

  const titleSeed = text || t('aiChat.imageConversationTitle')
  const conversation = activeConversation.value || (await createConversation(titleSeed))
  if (!conversation) return

  if (!activeConversation.value) {
    conversations.value = [conversation, ...conversations.value]
    activeConversationId.value = conversation.id
  }

  draft.value = ''
  selectedImages.value = []
  resetImageInput()
  sending.value = true
  abortController.value = new AbortController()

  const historyReferenceImageUrls =
    isImageModel(selectedModel.value) && images.length === 0
      ? latestImageModelReferenceUrls(conversation.id)
      : []
  if (historyReferenceImageUrls.length > 0) {
    imageContinuationHint.value = t('aiChat.continuingImageEdit')
    appStore.showInfo(t('aiChat.continuingImageEdit'))
  }
  const userContent = serializeChatContent(buildUserContent(text, images, historyReferenceImageUrls))
  const userMessage = makeLocalMessage(conversation.id, 'user', userContent)
  const assistantMessage = makeLocalMessage(conversation.id, 'assistant', '')
  appendLocalMessages(conversation.id, [userMessage, assistantMessage])
  await scrollToBottom()

  try {
    if (isImageModel(selectedModel.value)) {
      const imagePayload = {
        prompt: text,
        model: selectedModel.value,
        group_id: selectedGroupId.value,
        conversation_id: conversation.id
      }
      const result = images.length > 0
        ? await userAiAPI.editImages(
          {
            ...imagePayload,
            image_urls: images.map((image) => image.imageUrl)
          },
          {
            signal: abortController.value.signal
          }
        )
        : await userAiAPI.generateImages(
          imagePayload,
          {
            signal: abortController.value.signal
          }
        )
      const assistantImageContent = buildAssistantImageContent(result.data)
      if (assistantImageContent.length === 0) {
        throw new Error(t('aiImage.generateFailed'))
      }
      updateLocalMessageContent(conversation.id, assistantMessage.id, serializeChatContent(assistantImageContent))
      await scrollToBottom()
    } else {
      const requestMessages = buildRequestMessages(conversation.id, userMessage)
      const streamedContent = await userAiAPI.streamChatCompletions(
        {
          model: selectedModel.value,
          group_id: selectedGroupId.value,
          conversation_id: conversation.id,
          messages: requestMessages,
          stream: true
        },
        {
          signal: abortController.value.signal,
          onDelta: async (delta) => {
            appendAssistantDelta(conversation.id, assistantMessage.id, delta)
            await scrollToBottom()
          }
        }
      )
      if (!getLocalMessageContent(conversation.id, assistantMessage.id) && streamedContent) {
        updateLocalMessageContent(conversation.id, assistantMessage.id, streamedContent)
        await scrollToBottom()
      }
    }
    await loadConversations(conversation.id)
  } catch (err) {
    if (!isRequestCanceled(err)) {
      await loadConversations(conversation.id)
      if (!hasSavedAssistantReply(conversation.id, userContent)) {
        removeLocalMessage(conversation.id, assistantMessage.id)
        appStore.showError(extractApiErrorMessage(err, t('aiChat.sendFailed')))
      }
    }
  } finally {
    sending.value = false
    abortController.value = null
    imageContinuationHint.value = ''
    await nextTick()
    inputRef.value?.focus()
  }
}

function isRequestCanceled(err: unknown): boolean {
  const record = err as { name?: string; code?: string }
  return record?.name === 'AbortError' || record?.name === 'CanceledError' || record?.code === 'ERR_CANCELED'
}

function stopStreaming(): void {
  abortController.value?.abort()
  abortController.value = null
  sending.value = false
}

function makeLocalMessage(conversationId: number, role: string, content: string): AIChatMessage {
  return {
    id: tempMessageId--,
    conversation_id: conversationId,
    user_id: 0,
    role,
    content,
    model: selectedModel.value,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }
}

function appendLocalMessages(conversationId: number, messages: AIChatMessage[]): void {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  if (!conversation) return
  conversation.messages = [...conversation.messages, ...messages]
  conversation.updated_at = new Date().toISOString()
}

function removeLocalMessage(conversationId: number, messageId: number): void {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  if (!conversation) return
  conversation.messages = conversation.messages.filter((item) => item.id !== messageId)
}

function findLocalMessage(conversationId: number, messageId: number): AIChatMessage | null {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  return conversation?.messages.find((item) => item.id === messageId) || null
}

function getLocalMessageContent(conversationId: number, messageId: number): string {
  return findLocalMessage(conversationId, messageId)?.content || ''
}

function updateLocalMessageContent(conversationId: number, messageId: number, content: string): void {
  const message = findLocalMessage(conversationId, messageId)
  if (message) {
    message.content = content
  }
}

function appendAssistantDelta(conversationId: number, messageId: number, delta: string): void {
  const message = findLocalMessage(conversationId, messageId)
  if (message) {
    message.content = `${message.content || ''}${delta}`
  }
}

function hasSavedAssistantReply(conversationId: number, userContent: string): boolean {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const messages = conversation?.messages || []
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i]
    if (message.role !== 'user' || message.content.trim() !== userContent.trim()) continue
    return messages.slice(i + 1).some((item) => item.role === 'assistant' && Boolean(item.content.trim()))
  }
  return false
}

function buildRequestMessages(conversationId: number, userMessage: AIChatMessage): ChatCompletionMessage[] {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const persistedMessages = (conversation?.messages || [])
    .filter((message) => message.id > 0 || message.id === userMessage.id)
    .filter((message) => {
      if (!['system', 'user', 'assistant'].includes(message.role)) return false
      return messageRequestContent(message) !== null
    })
    .map((message) => ({
      role: message.role as 'system' | 'user' | 'assistant',
      content: messageRequestContent(message) as ChatMessageContent
    }))
  return persistedMessages
}

function messageRequestContent(message: AIChatMessage): ChatMessageContent | null {
  const parsed = parseMessageContent(message.content)
  if (!parsed.hasContent) return null
  if (message.role === 'assistant' && parsed.imageUrls.length > 0) {
    return null
  }
  return parsed.requestContent
}

function buildAssistantImageContent(images: Array<{ url?: string; b64_json?: string; revised_prompt?: string }>): Exclude<ChatMessageContent, string> {
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

function syncActiveConversationSelection(): void {
  applyAvailableModelSelection(activeConversation.value, modelsResult.value.default_group_id ?? modelsResult.value.groups[0]?.id ?? null, modelsResult.value.default_model || '')
}

function applyAvailableModelSelection(
  conversation: AIConversation | null,
  defaultGroupId: number | null,
  defaultModel: string
): void {
  const groups = modelsResult.value.groups || []
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

function openImagePreview(url: string, alt: string): void {
  previewImageUrl.value = url
  previewImageAlt.value = alt
}

function closeImagePreview(): void {
  previewImageUrl.value = ''
  previewImageAlt.value = ''
}

function openImagePicker(): void {
  if (sending.value || imageUploading.value || selectedImages.value.length >= maxSelectedImages) return
  imageInputRef.value?.click()
}

async function handleImageSelection(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  if (files.length === 0) return

  imageUploading.value = true
  try {
    for (const file of files) {
      if (selectedImages.value.length >= maxSelectedImages) {
        appStore.showError(t('aiChat.imageLimit', { count: maxSelectedImages }))
        break
      }
      if (!isAllowedImageFile(file)) {
        appStore.showError(t('aiChat.imageTypeError'))
        continue
      }
      if (file.size > maxOriginalImageBytes) {
        appStore.showError(t('aiChat.imageTooLarge'))
        continue
      }
      try {
        const compressed = await compressImageForChat(file)
        if (compressed.size > maxCompressedImageBytes) {
          appStore.showError(t('aiChat.imageCompressedTooLarge'))
          continue
        }
        const uploaded = await userAiAPI.uploadImage(compressed)
        const imageUrl = uploaded.image_url?.trim()
        if (!isAllowedUploadedImageUrl(imageUrl)) {
          appStore.showError(t('aiChat.imageUploadFailed'))
          continue
        }
        selectedImages.value = [
          ...selectedImages.value,
          {
            id: selectedImageId++,
            name: file.name,
            type: compressed.type,
            size: compressed.size,
            imageUrl
          }
        ]
      } catch {
        appStore.showError(t('aiChat.imageUploadFailed'))
      }
    }
  } finally {
    imageUploading.value = false
    resetImageInput()
  }
}

function removeSelectedImage(id: number): void {
  selectedImages.value = selectedImages.value.filter((image) => image.id !== id)
  resetImageInput()
}

function resetImageInput(): void {
  if (imageInputRef.value) {
    imageInputRef.value.value = ''
  }
}

function isAllowedImageFile(file: File): boolean {
  return file.type.startsWith('image/') && allowedImageTypes.has(file.type)
}

async function compressImageForChat(file: File): Promise<File> {
  const bitmap = await loadImageBitmap(file)
  try {
    const { width, height } = fitImageDimensions(bitmap.width, bitmap.height, maxImageDimension)
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      throw new Error('canvas context unavailable')
    }
    ctx.drawImage(bitmap, 0, 0, width, height)
    const blob = await canvasToJPEGBlob(canvas, imageJPEGQuality)
    const baseName = file.name.replace(/\.[^.]*$/, '') || 'image'
    return new File([blob], `${baseName}.jpg`, {
      type: 'image/jpeg',
      lastModified: Date.now()
    })
  } finally {
    if ('close' in bitmap && typeof bitmap.close === 'function') {
      bitmap.close()
    }
  }
}

async function loadImageBitmap(file: File): Promise<ImageBitmap> {
  if ('createImageBitmap' in window) {
    try {
      return await createImageBitmap(file)
    } catch {
      // Fall through for browsers that expose createImageBitmap but cannot decode this input.
    }
  }
  return loadImageBitmapWithElement(file)
}

function loadImageBitmapWithElement(file: File): Promise<ImageBitmap> {
  return new Promise((resolve, reject) => {
    const objectUrl = URL.createObjectURL(file)
    const image = new Image()
    image.onload = async () => {
      try {
        const canvas = document.createElement('canvas')
        canvas.width = image.naturalWidth
        canvas.height = image.naturalHeight
        const ctx = canvas.getContext('2d')
        if (!ctx) throw new Error('canvas context unavailable')
        ctx.drawImage(image, 0, 0)
        resolve(await createImageBitmap(canvas))
      } catch (err) {
        reject(err)
      } finally {
        URL.revokeObjectURL(objectUrl)
      }
    }
    image.onerror = () => {
      URL.revokeObjectURL(objectUrl)
      reject(new Error('failed to decode image'))
    }
    image.src = objectUrl
  })
}

function fitImageDimensions(width: number, height: number, maxDimension: number): { width: number; height: number } {
  if (width <= 0 || height <= 0) {
    throw new Error('invalid image dimensions')
  }
  const scale = Math.min(1, maxDimension / Math.max(width, height))
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale))
  }
}

function canvasToJPEGBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (!blob) {
          reject(new Error('failed to encode image'))
          return
        }
        resolve(blob)
      },
      'image/jpeg',
      quality
    )
  })
}

function buildUserContent(text: string, images: SelectedImage[], referenceImageUrls: string[] = []): ChatMessageContent {
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

function latestImageModelReferenceUrls(conversationId: number): string[] {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const messages = conversation?.messages || []
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role !== 'assistant') continue
    const imageUrls = messageImageUrls(messages[i]).filter(isHostedUserAIImageUrl)
    if (imageUrls.length > 0) return imageUrls.slice(0, maxSelectedImages)
  }
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role !== 'user') continue
    const imageUrls = messageImageUrls(messages[i]).filter(isHostedUserAIImageUrl)
    if (imageUrls.length > 0) return imageUrls.slice(0, maxSelectedImages)
  }
  return []
}

function serializeChatContent(content: ChatMessageContent): string {
  return typeof content === 'string' ? content : JSON.stringify(content)
}

function messageText(message: AIChatMessage): string {
  return parseMessageContent(message.content).text
}

function messageImageUrls(message: AIChatMessage): string[] {
  return parseMessageContent(message.content).imageUrls
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
          if (imageUrl) {
            if (isAllowedUploadedImageUrl(imageUrl)) {
              imageUrls.push(imageUrl)
              parts.push({
                type: 'image_url',
                image_url: {
                  url: imageUrl
                }
              })
            }
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

function extractImageUrl(record: Record<string, any>): string {
  const direct = typeof record.url === 'string' ? record.url : ''
  const nested =
    record.image_url && typeof record.image_url === 'object' && typeof record.image_url.url === 'string'
      ? record.image_url.url
      : ''
  const flat = typeof record.image_url === 'string' ? record.image_url : ''
  return String(nested || flat || direct).trim()
}

function isAllowedUploadedImageUrl(url: string): boolean {
  const trimmed = url.trim()
  return isAllowedImageDisplayUrl(trimmed)
}

function isAllowedImageDisplayUrl(url: string): boolean {
  const trimmed = url.trim()
  return (
    isHostedUserAIImageUrl(trimmed) ||
    /^https?:\/\//i.test(trimmed)
  )
}

function isHostedUserAIImageUrl(url: string): boolean {
  return /^\/uploads\/user_ai\/\d+\/(?:generated\/)?[A-Za-z0-9._-]+\.(?:jpg|jpeg|png|webp|gif)$/i.test(url.trim())
}

function isImageModel(model: string): boolean {
  const normalized = model.trim().toLowerCase()
  return normalized === 'gpt-image' || normalized.startsWith('gpt-image-') || normalized.startsWith('grok-image')
}

async function downloadImage(url: string, name: string): Promise<void> {
  const filename = `${name}.${fileExtensionFromImageURL(url)}`
  try {
    const response = await fetch(url, {
      credentials: url.startsWith('/') ? 'include' : 'omit'
    })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    const blob = await response.blob()
    const blobURL = URL.createObjectURL(blob)
    triggerDownload(blobURL, filename)
    window.setTimeout(() => URL.revokeObjectURL(blobURL), 1000)
  } catch {
    triggerDownload(url, filename, true)
  }
}

function triggerDownload(url: string, filename: string, newWindow = false): void {
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  if (newWindow) {
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
  }
  document.body.appendChild(link)
  link.click()
  link.remove()
}

function fileExtensionFromImageURL(url: string): string {
  const lower = url.toLowerCase()
  if (lower.includes('.jpeg') || lower.includes('.jpg')) return 'jpg'
  if (lower.includes('.webp')) return 'webp'
  if (lower.includes('.gif')) return 'gif'
  return 'png'
}

function conversationTitle(conversation: AIConversation): string {
  return conversation.title?.trim() || t('aiChat.untitled')
}

function renderMarkdown(content: string): string {
  const html = marked.parse(content || '', { async: false }) as string
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target', 'rel']
  })
}

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const el = messagesContainerRef.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}
</script>

<style scoped>
.ai-chat-shell {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  height: calc(100vh - 4rem - 2rem);
  height: calc(100dvh - 4rem - 2rem);
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  background: white;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
}

.mobile-chat-header {
  display: none;
}

.dark .ai-chat-shell {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

@media (min-width: 768px) {
  .ai-chat-shell {
    height: calc(100vh - 4rem - 3rem);
    height: calc(100dvh - 4rem - 3rem);
  }
}

@media (min-width: 1024px) {
  .ai-chat-shell {
    grid-template-columns: 18rem minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
    height: calc(100vh - 4rem - 4rem);
    height: calc(100dvh - 4rem - 4rem);
  }
}

.conversation-panel {
  display: flex;
  min-height: 16rem;
  max-height: 18rem;
  flex-direction: column;
  overflow: hidden;
  border-bottom: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
}

.dark .conversation-panel {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.6);
}

@media (min-width: 1024px) {
  .conversation-panel {
    min-height: 0;
    max-height: none;
    border-right: 1px solid rgb(229 231 235);
    border-bottom: 0;
  }

  .dark .conversation-panel {
    border-right-color: rgb(51 65 85);
  }
}

.conversation-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 1rem;
}

.conversation-list {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  padding: 0 0.75rem 0.75rem;
}

.conversation-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.625rem;
  border-radius: 0.75rem;
  padding: 0.75rem;
  color: rgb(55 65 81);
  transition: background-color 0.15s ease, color 0.15s ease;
}

.conversation-item:hover,
.conversation-item-active {
  background: white;
  color: rgb(17 24 39);
}

.dark .conversation-item {
  color: rgb(203 213 225);
}

.dark .conversation-item:hover,
.dark .conversation-item-active {
  background: rgb(30 41 59);
  color: white;
}

.conversation-delete {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: rgb(156 163 175);
}

.conversation-delete:hover {
  background: rgb(254 226 226);
  color: rgb(220 38 38);
}

.dark .conversation-delete:hover {
  background: rgb(127 29 29 / 0.35);
  color: rgb(248 113 113);
}

.empty-conversations {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem 1rem;
  text-align: center;
  font-size: 0.875rem;
  color: rgb(107 114 128);
}

.dark .empty-conversations {
  color: rgb(148 163 184);
}

.mobile-conversation-overlay,
.mobile-conversation-drawer {
  display: none;
}

.chat-panel {
  display: flex;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
}

.chat-toolbar {
  display: flex;
  flex-shrink: 0;
  flex-direction: column;
  gap: 1rem;
  border-bottom: 1px solid rgb(229 231 235);
  padding: 1rem;
}

.dark .chat-toolbar {
  border-color: rgb(51 65 85);
}

@media (min-width: 768px) {
  .chat-toolbar {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }
}

.chat-model-controls {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.75rem;
}

@media (min-width: 640px) {
  .chat-model-controls {
    grid-template-columns: 12rem minmax(14rem, 18rem);
  }
}

.control-label {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.375rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(75 85 99);
}

.dark .control-label {
  color: rgb(203 213 225);
}

.messages-panel {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  background: rgb(255 255 255);
  padding: 1rem;
}

.dark .messages-panel {
  background: rgb(15 23 42);
}

.message-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.message-row {
  display: flex;
}

.message-row-user {
  justify-content: flex-end;
}

.message-row-assistant {
  justify-content: flex-start;
}

.message-bubble {
  max-width: min(46rem, 92%);
  border-radius: 1rem;
  border: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
  padding: 0.875rem 1rem;
  color: rgb(31 41 55);
}

.message-row-user .message-bubble {
  border-color: rgb(37 99 235);
  background: rgb(37 99 235);
  color: white;
}

.dark .message-bubble {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
  color: rgb(226 232 240);
}

.dark .message-row-user .message-bubble {
  border-color: rgb(59 130 246);
  background: rgb(37 99 235);
  color: white;
}

.message-meta {
  margin-bottom: 0.5rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(107 114 128);
}

.message-row-user .message-meta {
  color: rgb(219 234 254);
}

.dark .message-meta {
  color: rgb(148 163 184);
}

.message-image-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.625rem;
}

.message-image-grid:empty {
  display: none;
}

.message-image-thumb {
  height: 7rem;
  width: 7rem;
  border-radius: 0.75rem;
  object-fit: cover;
  background: rgb(255 255 255 / 0.16);
}

.message-image-button {
  display: inline-flex;
  overflow: hidden;
  border-radius: 0.75rem;
  border: 1px solid rgb(191 219 254);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.message-image-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 25px rgb(15 23 42 / 0.18);
}

.message-row-user .message-image-thumb {
  border-color: rgb(147 197 253);
}

.message-row-user .message-image-button {
  border-color: rgb(147 197 253);
}

.assistant-image-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.assistant-image-card {
  display: flex;
  width: min(18rem, 100%);
  flex-direction: column;
  gap: 0.625rem;
}

.assistant-image-preview {
  display: block;
  overflow: hidden;
  border-radius: 0.875rem;
  border: 1px solid rgb(209 213 219);
  background: rgb(255 255 255);
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}

.assistant-image-preview:hover {
  transform: translateY(-1px);
  border-color: rgb(37 99 235);
  box-shadow: 0 12px 28px rgb(15 23 42 / 0.16);
}

.assistant-image-preview img {
  display: block;
  width: 100%;
  object-fit: cover;
}

.message-download-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  align-self: flex-start;
  border-radius: 999px;
  border: 1px solid rgb(209 213 219);
  background: rgb(255 255 255 / 0.92);
  padding: 0.425rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(55 65 81);
  transition: border-color 0.15s ease, color 0.15s ease, background-color 0.15s ease;
}

.message-download-button:hover {
  border-color: rgb(37 99 235);
  color: rgb(37 99 235);
}

.dark .assistant-image-preview {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

.dark .assistant-image-preview:hover {
  border-color: rgb(96 165 250);
  box-shadow: 0 12px 28px rgb(2 6 23 / 0.45);
}

.dark .message-download-button {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.88);
  color: rgb(226 232 240);
}

.dark .message-download-button:hover {
  border-color: rgb(96 165 250);
  color: rgb(147 197 253);
}

.image-preview-overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(2 6 23 / 0.8);
  padding: 1.5rem;
  backdrop-filter: blur(6px);
}

.image-preview-dialog {
  display: flex;
  max-height: 100%;
  max-width: min(72rem, 100%);
  flex-direction: column;
  gap: 0.875rem;
}

.image-preview-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}

.image-preview-toolbar-button {
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgb(148 163 184 / 0.45);
  background: rgb(15 23 42 / 0.88);
  color: white;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.image-preview-toolbar-button:hover {
  border-color: rgb(147 197 253);
  background: rgb(30 41 59 / 0.92);
}

.image-preview-full {
  max-height: calc(100vh - 7rem);
  max-width: min(72rem, 100%);
  border-radius: 1rem;
  object-fit: contain;
  box-shadow: 0 28px 60px rgb(2 6 23 / 0.45);
}

.empty-chat {
  display: flex;
  min-height: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  text-align: center;
  color: rgb(107 114 128);
}

.empty-chat h3 {
  font-size: 1rem;
  font-weight: 700;
  color: rgb(17 24 39);
}

.empty-chat p {
  max-width: 22rem;
  font-size: 0.875rem;
}

.empty-chat-icon {
  display: inline-flex;
  height: 3rem;
  width: 3rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.875rem;
  background: rgb(239 246 255);
  color: rgb(37 99 235);
}

.dark .empty-chat {
  color: rgb(148 163 184);
}

.dark .empty-chat h3 {
  color: white;
}

.dark .empty-chat-icon {
  background: rgb(30 58 138 / 0.35);
  color: rgb(147 197 253);
}

.composer {
  flex-shrink: 0;
  border-top: 1px solid rgb(229 231 235);
  padding: 1rem;
  background: rgb(249 250 251);
}

.dark .composer {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.75);
}

.composer-input {
  flex: 1;
  min-height: 5rem;
  resize: vertical;
}

.composer-editor {
  min-width: 0;
}

.image-continuation-hint {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  margin-bottom: 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(37 99 235);
}

.dark .image-continuation-hint {
  color: rgb(96 165 250);
}

.composer-input-row {
  display: flex;
  min-width: 0;
  align-items: stretch;
  gap: 0.625rem;
}

.image-upload-button {
  display: inline-flex;
  width: 2.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  border: 1px solid rgb(209 213 219);
  background: white;
  color: rgb(75 85 99);
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.image-upload-button:hover:not(:disabled) {
  border-color: rgb(37 99 235);
  color: rgb(37 99 235);
}

.image-upload-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.dark .image-upload-button {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
  color: rgb(203 213 225);
}

.dark .image-upload-button:hover:not(:disabled) {
  border-color: rgb(96 165 250);
  color: rgb(147 197 253);
}

.selected-image-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.625rem;
  margin-bottom: 0.75rem;
}

.image-upload-progress {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  margin-bottom: 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(75 85 99);
}

.dark .image-upload-progress {
  color: rgb(203 213 225);
}

.selected-image-item {
  position: relative;
  height: 4.75rem;
  width: 4.75rem;
  flex-shrink: 0;
  overflow: hidden;
  border-radius: 0.75rem;
  border: 1px solid rgb(209 213 219);
  background: white;
}

.dark .selected-image-item {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
}

.selected-image-thumb {
  height: 100%;
  width: 100%;
  object-fit: cover;
}

.selected-image-remove {
  position: absolute;
  top: 0.25rem;
  right: 0.25rem;
  display: inline-flex;
  height: 1.5rem;
  width: 1.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgb(17 24 39 / 0.72);
  color: white;
}

.selected-image-remove:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.composer-actions {
  margin-top: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.markdown-body {
  overflow-wrap: anywhere;
  line-height: 1.65;
}

.markdown-body :deep(p) {
  margin: 0.5rem 0;
}

.markdown-body :deep(p:first-child) {
  margin-top: 0;
}

.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}

.markdown-body :deep(ul) {
  list-style: disc;
}

.markdown-body :deep(ol) {
  list-style: decimal;
}

.markdown-body :deep(pre) {
  margin: 0.75rem 0;
  overflow-x: auto;
  border-radius: 0.75rem;
  background: rgb(17 24 39);
  padding: 0.875rem;
  color: rgb(243 244 246);
}

.markdown-body :deep(code) {
  border-radius: 0.375rem;
  background: rgb(229 231 235);
  padding: 0.125rem 0.25rem;
  font-size: 0.875em;
}

.markdown-body :deep(pre code) {
  background: transparent;
  padding: 0;
  color: inherit;
}

.markdown-body :deep(blockquote) {
  margin: 0.75rem 0;
  border-left: 3px solid rgb(209 213 219);
  padding-left: 0.75rem;
  color: rgb(75 85 99);
}

.dark .markdown-body :deep(code) {
  background: rgb(51 65 85);
}

.dark .markdown-body :deep(blockquote) {
  border-left-color: rgb(71 85 105);
  color: rgb(203 213 225);
}

@media (max-width: 767px) {
  .ai-chat-shell {
    height: calc(100vh - 4rem - 1px);
    height: calc(100dvh - 4rem - 1px);
    margin: -1rem;
    grid-template-rows: minmax(0, 1fr);
    border: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .conversation-panel,
  .chat-toolbar {
    display: none;
  }

  .mobile-chat-header {
    display: flex;
    flex-shrink: 0;
    flex-direction: column;
    gap: 0.625rem;
    border-bottom: 1px solid rgb(229 231 235);
    background: rgb(255 255 255 / 0.94);
    padding: 0.75rem 0.875rem 0.625rem;
    backdrop-filter: blur(16px);
  }

  .dark .mobile-chat-header {
    border-color: rgb(51 65 85);
    background: rgb(15 23 42 / 0.94);
  }

  .mobile-title-row {
    display: flex;
    min-width: 0;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .mobile-conversations-button {
    display: inline-flex;
    height: 2.25rem;
    flex-shrink: 0;
    align-items: center;
    gap: 0.375rem;
    border-radius: 999px;
    border: 1px solid rgb(209 213 219);
    padding: 0 0.75rem;
    font-size: 0.8125rem;
    font-weight: 700;
    color: rgb(55 65 81);
    background: white;
  }

  .dark .mobile-conversations-button {
    border-color: rgb(71 85 105);
    color: rgb(226 232 240);
    background: rgb(30 41 59);
  }

  .mobile-model-controls {
    display: grid;
    grid-template-columns: minmax(0, 0.92fr) minmax(0, 1.08fr);
    gap: 0.5rem;
  }

  .mobile-select-pill {
    display: grid;
    min-width: 0;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: 0.375rem;
    border-radius: 999px;
    border: 1px solid rgb(229 231 235);
    background: rgb(249 250 251);
    padding: 0.25rem 0.375rem 0.25rem 0.625rem;
    color: rgb(75 85 99);
  }

  .dark .mobile-select-pill {
    border-color: rgb(51 65 85);
    background: rgb(30 41 59 / 0.72);
    color: rgb(203 213 225);
  }

  .mobile-select-pill > span {
    min-width: 0;
    font-size: 0.6875rem;
    font-weight: 700;
    white-space: nowrap;
  }

  .mobile-select-pill :deep(.select-trigger) {
    min-height: 1.75rem;
    border: 0;
    background: transparent;
    padding: 0 0.125rem;
    box-shadow: none;
  }

  .mobile-select-pill :deep(.select-value) {
    font-size: 0.75rem;
    font-weight: 700;
    color: rgb(17 24 39);
  }

  .dark .mobile-select-pill :deep(.select-value) {
    color: rgb(248 250 252);
  }

  .chat-panel {
    min-height: 0;
    background: rgb(255 255 255);
  }

  .dark .chat-panel {
    background: rgb(15 23 42);
  }

  .messages-panel {
    flex: 1 1 auto;
    padding: 0.875rem 0.75rem 0.75rem;
  }

  .message-list {
    gap: 0.625rem;
  }

  .message-bubble {
    max-width: 85%;
    border-radius: 1rem;
    padding: 0.625rem 0.75rem;
    font-size: 0.9375rem;
    line-height: 1.55;
  }

  .message-row-user .message-bubble {
    border-bottom-right-radius: 0.375rem;
  }

  .message-row-assistant .message-bubble {
    border-bottom-left-radius: 0.375rem;
  }

  .message-meta {
    margin-bottom: 0.25rem;
    gap: 0.375rem;
    font-size: 0.6875rem;
  }

  .message-image-grid {
    gap: 0.375rem;
    margin-bottom: 0.5rem;
  }

  .message-image-thumb {
    height: 5.25rem;
    width: 5.25rem;
    border-radius: 0.625rem;
  }

  .empty-chat {
    min-height: 100%;
    gap: 0.375rem;
    padding: 0 1.5rem;
  }

  .empty-chat-icon {
    display: none;
  }

  .empty-chat h3 {
    font-size: 1rem;
  }

  .empty-chat p {
    max-width: 15rem;
    font-size: 0.8125rem;
  }

  .composer {
    border-top: 1px solid rgb(229 231 235);
    background: rgb(255 255 255 / 0.95);
    padding: 0.5rem 0.625rem calc(0.5rem + env(safe-area-inset-bottom));
    backdrop-filter: blur(16px);
  }

  .dark .composer {
    border-color: rgb(51 65 85);
    background: rgb(15 23 42 / 0.95);
  }

  .composer {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: 0.5rem;
  }

  .composer-editor {
    min-width: 0;
  }

  .composer-input-row {
    gap: 0.375rem;
  }

  .image-upload-button {
    width: 2.75rem;
    height: 2.75rem;
    border-radius: 999px;
  }

  .selected-image-strip {
    grid-column: 1 / -1;
    flex-wrap: nowrap;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
    overflow-x: auto;
    padding-bottom: 0.125rem;
  }

  .image-upload-progress {
    grid-column: 1 / -1;
    margin-bottom: 0.5rem;
  }

  .selected-image-item {
    height: 3.75rem;
    width: 3.75rem;
    border-radius: 0.625rem;
  }

  .composer-input {
    height: 2.75rem;
    min-height: 2.75rem;
    max-height: 5rem;
    resize: none;
    border-radius: 1.375rem;
    padding: 0.6875rem 0.875rem;
    line-height: 1.35;
  }

  .composer-actions {
    margin-top: 0;
    display: contents;
  }

  .composer-actions > p {
    display: none;
  }

  .composer-actions > div {
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }

  .composer-actions .btn {
    height: 2.75rem;
    min-width: 2.75rem;
    border-radius: 999px;
    padding: 0;
  }

  .composer-actions .btn span {
    display: none;
  }

  .markdown-body {
    line-height: 1.55;
  }

  .mobile-conversation-overlay {
    position: fixed;
    inset: 0;
    z-index: 100000030;
    display: block;
    background: rgb(15 23 42 / 0.42);
  }

  .mobile-conversation-drawer {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: 100000031;
    display: flex;
    width: min(22rem, 88vw);
    flex-direction: column;
    border-right: 1px solid rgb(229 231 235);
    background: white;
    box-shadow: 18px 0 40px rgb(15 23 42 / 0.18);
  }

  .dark .mobile-conversation-drawer {
    border-color: rgb(51 65 85);
    background: rgb(15 23 42);
  }

  .mobile-drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    border-bottom: 1px solid rgb(229 231 235);
    padding: calc(0.875rem + env(safe-area-inset-top)) 1rem 0.875rem;
  }

  .dark .mobile-drawer-header {
    border-color: rgb(51 65 85);
  }

  .mobile-drawer-header h2 {
    font-size: 1rem;
    font-weight: 800;
    color: rgb(17 24 39);
  }

  .mobile-drawer-header p {
    margin-top: 0.125rem;
    font-size: 0.75rem;
    color: rgb(107 114 128);
  }

  .dark .mobile-drawer-header h2 {
    color: white;
  }

  .dark .mobile-drawer-header p {
    color: rgb(148 163 184);
  }

  .mobile-drawer-close {
    display: inline-flex;
    height: 2rem;
    width: 2rem;
    align-items: center;
    justify-content: center;
    border-radius: 999px;
    color: rgb(75 85 99);
    background: rgb(243 244 246);
  }

  .dark .mobile-drawer-close {
    color: rgb(226 232 240);
    background: rgb(30 41 59);
  }

  .mobile-drawer-actions {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 0.5rem;
    padding: 0.75rem;
  }

  .mobile-drawer-actions .btn-primary {
    justify-content: center;
  }

  .mobile-drawer-list {
    min-height: 0;
    flex: 1;
    overflow-y: auto;
    padding: 0 0.75rem 1rem;
  }

  .mobile-drawer-fade-enter-active,
  .mobile-drawer-fade-leave-active,
  .mobile-drawer-slide-enter-active,
  .mobile-drawer-slide-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
  }

  .mobile-drawer-fade-enter-from,
  .mobile-drawer-fade-leave-to {
    opacity: 0;
  }

  .mobile-drawer-slide-enter-from,
  .mobile-drawer-slide-leave-to {
    transform: translateX(-100%);
  }
}
</style>
