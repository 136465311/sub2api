<template>
  <AppLayout>
    <div class="recharge-balance-layout">
      <div class="card flex-1 min-h-0 overflow-hidden">
        <div class="recharge-balance-shell">
          <a
            :href="embeddedUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm recharge-open-fab"
          >
            <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
            {{ t('customPage.openInNewTab') }}
          </a>
          <iframe
            :src="embeddedUrl"
            class="recharge-balance-frame"
            allowfullscreen
          ></iframe>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'

const RECHARGE_BALANCE_URL = 'https://pay.ldxp.cn/shop/B231XR9U'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const pageTheme = ref<'light' | 'dark'>('light')
let themeObserver: MutationObserver | null = null

const embeddedUrl = computed(() =>
  buildEmbeddedUrl(
    RECHARGE_BALANCE_URL,
    authStore.user?.id,
    authStore.token,
    pageTheme.value,
    locale.value,
  ),
)

onMounted(() => {
  pageTheme.value = detectTheme()

  if (typeof document !== 'undefined') {
    themeObserver = new MutationObserver(() => {
      pageTheme.value = detectTheme()
    })
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
  }
})

onUnmounted(() => {
  if (themeObserver) {
    themeObserver.disconnect()
    themeObserver = null
  }
})
</script>

<style scoped>
.recharge-balance-layout {
  @apply flex flex-col;
  height: calc(100vh - 64px - 4rem);
}

.recharge-balance-shell {
  @apply relative h-full w-full overflow-hidden rounded-2xl bg-white dark:bg-dark-900;
}

.recharge-open-fab {
  @apply absolute right-3 top-3 z-10 shadow-sm backdrop-blur supports-[backdrop-filter]:bg-white/80 dark:supports-[backdrop-filter]:bg-dark-800/80;
}

.recharge-balance-frame {
  display: block;
  margin: 0;
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
  box-shadow: none;
  background: transparent;
}
</style>
