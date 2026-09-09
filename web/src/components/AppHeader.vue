<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { setLocale } from '@/i18n'
import BrandLogo from '@/components/BrandLogo.vue'

const { t, locale } = useI18n()

function toggle() {
  setLocale(locale.value === 'ru' ? 'en' : 'ru')
}
</script>

<template>
  <header class="fixed top-0 inset-x-0 z-50 border-b border-base-content/10 bg-base-100/95 backdrop-blur-xl">
    <div class="flex items-center justify-between gap-4 max-w-[90rem] mx-auto px-3 sm:px-5 min-h-14">
      <BrandLogo />
      <button class="btn btn-ghost btn-sm font-mono" type="button" @click="toggle">
        {{ locale === 'ru' ? t('lang.en') : t('lang.ru') }}
      </button>
    </div>
  </header>
</template>
