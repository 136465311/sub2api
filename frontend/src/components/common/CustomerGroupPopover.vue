<template>
  <div class="group relative">
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white dark:focus-visible:ring-offset-dark-900"
      :aria-label="t('customerGroup.title')"
    >
      <Icon name="users" size="sm" />
      <span class="hidden sm:inline">{{ t('customerGroup.title') }}</span>
    </button>

    <div
      class="pointer-events-none absolute left-1/2 top-full z-50 w-[260px] -translate-x-1/2 pt-3 opacity-0 transition-all duration-150 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100 sm:w-[272px]"
    >
      <div
        class="rounded-lg border border-gray-200 bg-white px-6 py-7 text-center shadow-lg shadow-gray-900/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
      >
        <img
          src="/customer-group-qr.jpg"
          :alt="t('customerGroup.qrAlt')"
          class="mx-auto h-36 w-36 rounded-md object-contain sm:h-40 sm:w-40"
        />
        <div class="mt-5 text-xs font-medium text-gray-500 dark:text-dark-400">
          QQ
        </div>
        <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
          {{ t('customerGroup.scanToJoin') }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
</script>
