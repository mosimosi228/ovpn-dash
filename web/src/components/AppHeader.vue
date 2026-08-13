<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { setLocale } from '@/i18n'

const { t, locale } = useI18n()

function toggle() {
  setLocale(locale.value === 'ru' ? 'en' : 'ru')
}
</script>

<template>
  <header class="w-full max-w-6xl mx-auto px-4 sm:px-6 pt-6 flex items-center justify-between">
    <div>
      <div class="font-display text-2xl tracking-tight text-base-content">{{ t('app.title') }}</div>
      <div class="text-xs uppercase tracking-[0.2em] text-primary/80 mt-1">{{ t('app.subtitle') }}</div>
    </div>
    <button class="btn btn-ghost btn-sm font-mono" type="button" @click="toggle">
      {{ locale === 'ru' ? t('lang.en') : t('lang.ru') }}
    </button>
  </header>
</template>
