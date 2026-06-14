<template>
  <AppLayout>
    <template #header>
      <div class="flex flex-col gap-1">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('aiImage.title') }}</h1>
        <p class="text-sm text-gray-600 dark:text-dark-400">{{ t('aiImage.description') }}</p>
      </div>
    </template>

    <div class="ai-image-page">
      <section class="generator-shell">
        <form class="generation-panel" @submit.prevent="generateImages">
          <div class="panel-heading">
            <div>
              <h2>{{ t('aiImage.title') }}</h2>
              <p>{{ selectedModelLabel || t('aiImage.selectModel') }}</p>
            </div>
            <button
              type="button"
              class="icon-button"
              :title="t('common.refresh')"
              :disabled="loadingModels || generating"
              @click="loadModels"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingModels }" />
            </button>
          </div>

          <label class="field-block">
            <span>{{ t('aiImage.prompt') }}</span>
            <textarea
              v-model="prompt"
              class="input prompt-input"
              rows="7"
              :placeholder="t('aiImage.promptPlaceholder')"
              :disabled="generating"
            ></textarea>
          </label>

          <label class="field-block">
            <span>{{ t('aiImage.model') }}</span>
            <Select
              v-model="selectedModelKey"
              :options="modelOptions"
              :placeholder="loadingModels ? t('aiChat.loadingModels') : t('aiImage.selectModel')"
              :disabled="loadingModels || generating || modelOptions.length === 0"
              :searchable="true"
              :empty-text="t('aiImage.noModels')"
            />
          </label>

          <div class="field-block">
            <span>{{ t('aiImage.size') }}</span>
            <div class="size-selector" role="group" :aria-label="t('aiImage.size')">
              <button
                v-for="option in sizeOptions"
                :key="option.value"
                type="button"
                class="size-option"
                :class="{ 'size-option-active': selectedSize === option.value }"
                :disabled="generating"
                @click="selectedSize = option.value"
              >
                <span class="ratio-mark" :class="option.markClass"></span>
                <span>{{ option.label }}</span>
              </button>
            </div>
          </div>

          <label class="field-block">
            <span>{{ t('aiImage.quantity') }}</span>
            <Select
              v-model="selectedCount"
              :options="countOptions"
              :disabled="generating"
            />
          </label>

          <button type="submit" class="btn btn-primary generate-button" :disabled="!canGenerate">
            <Icon v-if="generating" name="refresh" size="sm" class="animate-spin" />
            <Icon v-else name="sparkles" size="sm" />
            <span>{{ generating ? t('aiImage.generating') : t('aiImage.generate') }}</span>
          </button>
        </form>

        <section class="result-panel">
          <div class="panel-heading result-heading">
            <div>
              <h2>{{ t('aiImage.latest') }}</h2>
              <p>{{ latestCreatedAt ? t('aiImage.generatedAt', { time: formatRelativeTime(latestCreatedAt) }) : t('aiImage.latestEmpty') }}</p>
            </div>
          </div>

          <div v-if="generating" class="result-grid">
            <div
              v-for="index in selectedCountNumber"
              :key="`loading-${index}`"
              class="image-card image-card-loading"
              :class="ratioClassFromSize(selectedSize)"
            >
              <Icon name="refresh" size="lg" class="animate-spin" />
            </div>
          </div>

          <div v-else-if="latestImages.length" class="result-grid">
            <article
              v-for="(image, index) in latestImages"
              :key="`${image.url}-${index}`"
              class="image-card"
              :class="ratioClassFromSize(selectedSize)"
            >
              <img :src="image.url" :alt="t('aiImage.resultImageAlt', { count: index + 1 })" loading="lazy" />
              <div class="image-actions">
                <button
                  type="button"
                  class="download-button"
                  :title="t('aiImage.download')"
                  @click="downloadImage(image.url, `ai-image-${latestCreatedAt || 'latest'}-${index + 1}`)"
                >
                  <Icon name="download" size="sm" />
                  <span>{{ t('aiImage.download') }}</span>
                </button>
              </div>
              <p v-if="image.revisedPrompt" class="revised-prompt">
                {{ image.revisedPrompt }}
              </p>
            </article>
          </div>

          <div v-else class="empty-state">
            <Icon name="sparkles" size="xl" />
            <span>{{ t('aiImage.latestEmpty') }}</span>
          </div>
        </section>
      </section>

      <section class="history-section">
        <header class="history-header">
          <div>
            <h2>{{ t('aiImage.history') }}</h2>
            <p>{{ history.length }} / {{ historyTotal }}</p>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            :disabled="historyLoading"
            @click="loadHistory(true)"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': historyLoading }" />
            <span>{{ t('aiImage.refreshHistory') }}</span>
          </button>
        </header>

        <div v-if="historyLoading && history.length === 0" class="history-grid">
          <div v-for="index in 4" :key="`history-loading-${index}`" class="history-card history-card-loading"></div>
        </div>

        <div v-else-if="history.length" class="history-grid">
          <article v-for="item in history" :key="item.id" class="history-card">
            <div class="history-image-strip">
              <div
                v-for="(image, index) in item.images"
                :key="`${item.id}-${index}`"
                class="history-image"
                :class="ratioClassFromSize(item.size)"
              >
                <img :src="image" :alt="t('aiImage.historyImageAlt', { count: index + 1 })" loading="lazy" />
                <button
                  type="button"
                  class="history-download"
                  :title="t('aiImage.download')"
                  @click="downloadImage(image, `ai-image-${item.id}-${index + 1}`)"
                >
                  <Icon name="download" size="sm" />
                </button>
              </div>
            </div>
            <div class="history-body">
              <p class="history-prompt">{{ item.prompt }}</p>
              <div class="history-meta">
                <span>{{ item.model }}</span>
                <span>{{ displaySize(item.size) }}</span>
                <span>{{ formatRelativeTime(item.created_at) }}</span>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-state history-empty">
          <Icon name="clock" size="xl" />
          <span>{{ t('aiImage.historyEmpty') }}</span>
        </div>

        <div v-if="hasMoreHistory" class="load-more-row">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="historyLoadingMore"
            @click="loadHistory(false)"
          >
            <Icon v-if="historyLoadingMore" name="refresh" size="sm" class="animate-spin" />
            <span>{{ t('aiImage.loadMore') }}</span>
          </button>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatRelativeTime } from '@/utils/format'
import {
  userAiAPI,
  type AIImageGenerationImage,
  type AIImageHistoryItem,
  type AIModelGroup
} from '@/api/userAi'

type ImageSize = '1:1' | '16:9' | '9:16'

interface FlatModelOption {
  value: string
  label: string
  groupId: number
  groupName: string
  model: string
  [key: string]: unknown
}

interface DisplayImage {
  url: string
  revisedPrompt: string
}

const { t } = useI18n()
const appStore = useAppStore()

const prompt = ref('')
const selectedModelKey = ref<string | null>(null)
const selectedSize = ref<ImageSize>('1:1')
const selectedCount = ref<number>(1)
const groups = ref<AIModelGroup[]>([])
const defaultGroupId = ref<number | null>(null)
const defaultModel = ref('')
const loadingModels = ref(false)
const generating = ref(false)
const latestImages = ref<DisplayImage[]>([])
const latestCreatedAt = ref('')
const history = ref<AIImageHistoryItem[]>([])
const historyPage = ref(1)
const historyTotal = ref(0)
const historyLoading = ref(false)
const historyLoadingMore = ref(false)

const sizeOptions: Array<{ value: ImageSize; label: string; markClass: string }> = [
  { value: '1:1', label: t('aiImage.sizeSquare'), markClass: 'ratio-square' },
  { value: '16:9', label: t('aiImage.sizeLandscape'), markClass: 'ratio-landscape' },
  { value: '9:16', label: t('aiImage.sizePortrait'), markClass: 'ratio-portrait' }
]

const countOptions = computed(() => [
  { value: 1, label: t('aiImage.n1') },
  { value: 2, label: t('aiImage.n2') },
  { value: 3, label: t('aiImage.n3') },
  { value: 4, label: t('aiImage.n4') }
])

const modelOptions = computed<FlatModelOption[]>(() =>
  groups.value.flatMap((group) =>
    (group.models || []).map((model) => ({
      value: `${group.id}:${model}`,
      label: `${model} / ${group.name}`,
      groupId: group.id,
      groupName: group.name,
      model
    }))
  )
)

const selectedModelOption = computed(() =>
  modelOptions.value.find((item) => item.value === selectedModelKey.value) || null
)

const selectedModelLabel = computed(() => selectedModelOption.value?.label || '')

const selectedCountNumber = computed(() => {
  const value = Number(selectedCount.value)
  return Number.isFinite(value) && value > 0 ? Math.min(4, Math.floor(value)) : 1
})

const canGenerate = computed(() =>
  Boolean(prompt.value.trim() && selectedModelOption.value && !generating.value)
)

const hasMoreHistory = computed(() => history.value.length < historyTotal.value)

watch(modelOptions, (options) => {
  if (options.length === 0) {
    selectedModelKey.value = null
    return
  }
  if (selectedModelKey.value && options.some((item) => item.value === selectedModelKey.value)) {
    return
  }
  const preferred = options.find((item) => item.groupId === defaultGroupId.value && item.model === defaultModel.value)
  selectedModelKey.value = (preferred || options[0]).value
})

onMounted(() => {
  void loadModels()
  void loadHistory(true)
})

async function loadModels(): Promise<void> {
  loadingModels.value = true
  try {
    const result = await userAiAPI.getImageModels()
    groups.value = result.groups || []
    defaultGroupId.value = result.default_group_id ?? null
    defaultModel.value = result.default_model || ''
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('aiImage.loadFailed')))
  } finally {
    loadingModels.value = false
  }
}

async function loadHistory(reset: boolean): Promise<void> {
  if (reset) {
    historyLoading.value = true
    historyPage.value = 1
  } else {
    historyLoadingMore.value = true
  }

  try {
    const page = reset ? 1 : historyPage.value + 1
    const result = await userAiAPI.listImageHistory(page, 20)
    history.value = reset ? result.items : [...history.value, ...result.items]
    historyPage.value = result.page
    historyTotal.value = result.total
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('aiImage.loadFailed')))
  } finally {
    historyLoading.value = false
    historyLoadingMore.value = false
  }
}

async function generateImages(): Promise<void> {
  if (!canGenerate.value || !selectedModelOption.value) return

  generating.value = true
  try {
    const selection = selectedModelOption.value
    const result = await userAiAPI.generateImages({
      prompt: prompt.value.trim(),
      model: selection.model,
      group_id: selection.groupId,
      size: selectedSize.value,
      n: selectedCountNumber.value
    })

    latestImages.value = result.data.map(imageToDisplay).filter((item): item is DisplayImage => Boolean(item))
    latestCreatedAt.value = result.created > 0 ? new Date(result.created * 1000).toISOString() : new Date().toISOString()
    appStore.showSuccess(t('aiImage.generateSuccess'))
    void loadHistory(true)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('aiImage.generateFailed')))
  } finally {
    generating.value = false
  }
}

function imageToDisplay(image: AIImageGenerationImage): DisplayImage | null {
  const url = image.url || (image.b64_json ? `data:image/png;base64,${image.b64_json}` : '')
  if (!url) return null
  return {
    url,
    revisedPrompt: image.revised_prompt || ''
  }
}

function ratioClassFromSize(size: string): string {
  const normalized = String(size || '').toLowerCase()
  if (normalized.includes('16:9') || normalized.includes('2048x1152')) return 'ratio-frame-landscape'
  if (normalized.includes('9:16') || normalized.includes('1152x2048')) return 'ratio-frame-portrait'
  return 'ratio-frame-square'
}

function displaySize(size: string): string {
  const normalized = String(size || '').toLowerCase()
  if (normalized.includes('2048x1152')) return t('aiImage.sizeLandscape')
  if (normalized.includes('1152x2048')) return t('aiImage.sizePortrait')
  return t('aiImage.sizeSquare')
}

async function downloadImage(url: string, name: string): Promise<void> {
  const filename = `${name}.${fileExtensionFromImageURL(url)}`
  try {
    if (url.startsWith('data:')) {
      triggerDownload(url, filename)
      return
    }

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
  if (lower.startsWith('data:image/jpeg') || lower.includes('.jpeg') || lower.includes('.jpg')) return 'jpg'
  if (lower.startsWith('data:image/webp') || lower.includes('.webp')) return 'webp'
  if (lower.startsWith('data:image/gif') || lower.includes('.gif')) return 'gif'
  return 'png'
}
</script>

<style scoped>
.ai-image-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.generator-shell {
  display: grid;
  grid-template-columns: minmax(18rem, 24rem) minmax(0, 1fr);
  gap: 1rem;
  align-items: stretch;
}

.generation-panel,
.result-panel,
.history-section {
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: white;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
}

.dark .generation-panel,
.dark .result-panel,
.dark .history-section {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

.generation-panel {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1rem;
}

.panel-heading,
.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.panel-heading h2,
.history-header h2 {
  font-size: 1rem;
  font-weight: 800;
  color: rgb(17 24 39);
}

.panel-heading p,
.history-header p {
  margin-top: 0.125rem;
  max-width: 24rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

.dark .panel-heading h2,
.dark .history-header h2 {
  color: white;
}

.dark .panel-heading p,
.dark .history-header p {
  color: rgb(148 163 184);
}

.icon-button {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  border: 1px solid rgb(229 231 235);
  color: rgb(75 85 99);
}

.icon-button:hover:not(:disabled) {
  border-color: rgb(37 99 235);
  color: rgb(37 99 235);
}

.icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.dark .icon-button {
  border-color: rgb(51 65 85);
  color: rgb(203 213 225);
}

.field-block {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.5rem;
  font-size: 0.8125rem;
  font-weight: 700;
  color: rgb(55 65 81);
}

.dark .field-block {
  color: rgb(203 213 225);
}

.prompt-input {
  min-height: 11rem;
  resize: vertical;
  line-height: 1.55;
}

.size-selector {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
}

.size-option {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
  padding: 0.625rem 0.5rem;
  font-size: 0.8125rem;
  font-weight: 800;
  color: rgb(55 65 81);
}

.size-option-active {
  border-color: rgb(37 99 235);
  background: rgb(239 246 255);
  color: rgb(29 78 216);
}

.dark .size-option {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
  color: rgb(226 232 240);
}

.dark .size-option-active {
  border-color: rgb(96 165 250);
  background: rgb(30 64 175 / 0.28);
  color: rgb(191 219 254);
}

.ratio-mark {
  display: block;
  flex-shrink: 0;
  border: 2px solid currentColor;
  border-radius: 0.1875rem;
}

.ratio-square {
  height: 0.875rem;
  width: 0.875rem;
}

.ratio-landscape {
  height: 0.625rem;
  width: 1rem;
}

.ratio-portrait {
  height: 1rem;
  width: 0.625rem;
}

.generate-button {
  justify-content: center;
  min-height: 2.75rem;
}

.result-panel {
  min-width: 0;
  padding: 1rem;
}

.result-heading {
  margin-bottom: 1rem;
}

.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 16rem), 1fr));
  gap: 0.875rem;
}

.image-card {
  position: relative;
  overflow: hidden;
  border-radius: 0.5rem;
  border: 1px solid rgb(229 231 235);
  background: rgb(243 244 246);
}

.dark .image-card {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
}

.image-card img,
.history-image img {
  height: 100%;
  width: 100%;
  object-fit: cover;
}

.ratio-frame-square {
  aspect-ratio: 1 / 1;
}

.ratio-frame-landscape {
  aspect-ratio: 16 / 9;
}

.ratio-frame-portrait {
  aspect-ratio: 9 / 16;
}

.image-card-loading,
.history-card-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgb(37 99 235);
  background:
    linear-gradient(90deg, rgb(243 244 246), rgb(229 231 235), rgb(243 244 246));
  background-size: 200% 100%;
  animation: image-shimmer 1.2s ease-in-out infinite;
}

.dark .image-card-loading,
.dark .history-card-loading {
  color: rgb(147 197 253);
  background:
    linear-gradient(90deg, rgb(30 41 59), rgb(51 65 85), rgb(30 41 59));
  background-size: 200% 100%;
}

.image-actions {
  position: absolute;
  inset: auto 0 0 0;
  display: flex;
  justify-content: flex-end;
  padding: 0.625rem;
  background: linear-gradient(to top, rgb(0 0 0 / 0.58), transparent);
}

.download-button,
.history-download {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border-radius: 0.5rem;
  background: rgb(255 255 255 / 0.92);
  color: rgb(17 24 39);
  font-size: 0.75rem;
  font-weight: 800;
}

.download-button {
  min-height: 2.125rem;
  padding: 0 0.75rem;
}

.history-download {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  height: 2rem;
  width: 2rem;
}

.download-button:hover,
.history-download:hover {
  background: white;
}

.revised-prompt {
  position: absolute;
  inset: auto 0 0 0;
  max-height: 4.5rem;
  overflow: hidden;
  padding: 0.625rem;
  background: rgb(15 23 42 / 0.74);
  color: white;
  font-size: 0.75rem;
  line-height: 1.35;
}

.image-actions + .revised-prompt {
  display: none;
}

.empty-state {
  display: flex;
  min-height: 18rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border: 1px dashed rgb(209 213 219);
  border-radius: 0.5rem;
  color: rgb(107 114 128);
  text-align: center;
}

.dark .empty-state {
  border-color: rgb(51 65 85);
  color: rgb(148 163 184);
}

.history-section {
  padding: 1rem;
}

.history-header {
  margin-bottom: 1rem;
}

.history-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 18rem), 1fr));
  gap: 0.875rem;
}

.history-card {
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
}

.dark .history-card {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

.history-card-loading {
  min-height: 16rem;
}

.history-image-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
  gap: 0.375rem;
  padding: 0.375rem;
  background: rgb(249 250 251);
}

.dark .history-image-strip {
  background: rgb(2 6 23 / 0.35);
}

.history-image {
  position: relative;
  overflow: hidden;
  border-radius: 0.375rem;
  background: rgb(243 244 246);
}

.history-body {
  padding: 0.75rem;
}

.history-prompt {
  display: -webkit-box;
  min-height: 2.75rem;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.45;
  color: rgb(17 24 39);
}

.dark .history-prompt {
  color: white;
}

.history-meta {
  margin-top: 0.625rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

.history-meta span {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-radius: 999px;
  background: rgb(243 244 246);
  padding: 0.1875rem 0.5rem;
}

.dark .history-meta {
  color: rgb(203 213 225);
}

.dark .history-meta span {
  background: rgb(30 41 59);
}

.history-empty {
  min-height: 14rem;
}

.load-more-row {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
}

@keyframes image-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

@media (max-width: 1023px) {
  .generator-shell {
    grid-template-columns: minmax(0, 1fr);
  }

  .generation-panel {
    order: 1;
  }

  .result-panel {
    order: 2;
  }
}

@media (max-width: 640px) {
  .ai-image-page {
    margin: -0.25rem;
    gap: 0.75rem;
  }

  .generation-panel,
  .result-panel,
  .history-section {
    padding: 0.75rem;
  }

  .size-selector {
    grid-template-columns: minmax(0, 1fr);
  }

  .download-button span,
  .history-header .btn span {
    display: none;
  }

  .result-grid,
  .history-grid {
    gap: 0.75rem;
  }

  .history-image-strip {
    grid-template-columns: repeat(auto-fit, minmax(6rem, 1fr));
  }
}
</style>
