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
                <div
                  v-if="message.role === 'assistant'"
                  class="markdown-body"
                  v-html="renderMarkdown(message.content || (sending ? t('aiChat.streaming') : ''))"
                ></div>
                <p v-else class="whitespace-pre-wrap break-words">{{ message.content }}</p>
              </div>
            </article>
          </div>
        </main>

        <footer class="composer">
          <textarea
            ref="inputRef"
            v-model="draft"
            class="input composer-input"
            rows="3"
            :placeholder="t('aiChat.inputPlaceholder')"
            :disabled="sending || !selectedModel"
            @keydown.enter.exact.prevent="sendMessage"
          ></textarea>
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
import { userAiAPI, type AIConversation, type AIChatMessage, type AIModelGroup } from '@/api/userAi'
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
const loadingModels = ref(false)
const loadingConversations = ref(false)
const sending = ref(false)
const pendingDeleteId = ref<number | null>(null)
const abortController = ref<AbortController | null>(null)
const messagesContainerRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)
let tempMessageId = -1

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

const canSend = computed(() => {
  return Boolean(draft.value.trim() && selectedModel.value && selectedGroupId.value && !sending.value)
})

watch(selectedGroupId, () => {
  const models = selectedGroup.value?.models || []
  if (!models.includes(selectedModel.value)) {
    selectedModel.value = models[0] || ''
  }
})

watch(activeConversationId, () => {
  const conversation = activeConversation.value
  if (!conversation) return
  if (conversation.group_id && modelsResult.value.groups.some((group) => group.id === conversation.group_id)) {
    selectedGroupId.value = conversation.group_id
  }
  if (conversation.model) {
    selectedModel.value = conversation.model
  }
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
    const result = await userAiAPI.getModels()
    modelsResult.value = {
      groups: result.groups || [],
      default_group_id: result.default_group_id,
      default_model: result.default_model
    }

    const defaultGroup = result.default_group_id ?? result.groups?.[0]?.id ?? null
    selectedGroupId.value = selectedGroupId.value ?? defaultGroup

    const models = selectedGroup.value?.models || []
    selectedModel.value = result.default_model || selectedModel.value || models[0] || ''
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
  const content = draft.value.trim()
  if (!content || !selectedModel.value || !selectedGroupId.value || sending.value) return

  const conversation = activeConversation.value || (await createConversation(content))
  if (!conversation) return

  if (!activeConversation.value) {
    conversations.value = [conversation, ...conversations.value]
    activeConversationId.value = conversation.id
  }

  draft.value = ''
  sending.value = true
  abortController.value = new AbortController()

  const userMessage = makeLocalMessage(conversation.id, 'user', content)
  const assistantMessage = makeLocalMessage(conversation.id, 'assistant', '')
  appendLocalMessages(conversation.id, [userMessage, assistantMessage])
  await scrollToBottom()

  try {
    const requestMessages = buildRequestMessages(conversation.id, userMessage)
    await userAiAPI.streamChatCompletions(
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
          assistantMessage.content += delta
          await scrollToBottom()
        }
      }
    )
    await loadConversations(conversation.id)
  } catch (err) {
    if ((err as Error)?.name !== 'AbortError') {
      removeLocalMessage(conversation.id, assistantMessage.id)
      appStore.showError(extractApiErrorMessage(err, t('aiChat.sendFailed')))
    }
  } finally {
    sending.value = false
    abortController.value = null
    await nextTick()
    inputRef.value?.focus()
  }
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

function buildRequestMessages(conversationId: number, userMessage: AIChatMessage) {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const persistedMessages = (conversation?.messages || [])
    .filter((message) => message.id > 0 || message.id === userMessage.id)
    .filter((message) => ['system', 'user', 'assistant'].includes(message.role) && message.content.trim())
    .map((message) => ({
      role: message.role as 'system' | 'user' | 'assistant',
      content: message.content
    }))
  return persistedMessages
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
  min-height: calc(100vh - 9rem);
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  background: white;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
}

.dark .ai-chat-shell {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

@media (min-width: 1024px) {
  .ai-chat-shell {
    grid-template-columns: 18rem minmax(0, 1fr);
  }
}

.conversation-panel {
  display: flex;
  min-height: 16rem;
  max-height: 18rem;
  flex-direction: column;
  border-bottom: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
}

.dark .conversation-panel {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.6);
}

@media (min-width: 1024px) {
  .conversation-panel {
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

.chat-panel {
  display: flex;
  min-height: 34rem;
  min-width: 0;
  flex-direction: column;
}

.chat-toolbar {
  display: flex;
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
  border-top: 1px solid rgb(229 231 235);
  padding: 1rem;
  background: rgb(249 250 251);
}

.dark .composer {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.75);
}

.composer-input {
  min-height: 5rem;
  resize: vertical;
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
</style>
