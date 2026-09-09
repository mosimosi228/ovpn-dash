<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
const { t } = useI18n()
const favicon = `${import.meta.env.BASE_URL}favicon.svg`
</script>

<template>
  <div class="flex items-center gap-2.5 shrink-0">
    <img
      :src="favicon"
      alt=""
      class="size-8 w-8 h-8 shrink-0 rounded-lg shadow-sm object-cover"
      width="32"
      height="32"
    />
    <div v-if="!compact" class="hidden min-[420px]:block leading-tight">
      <div class="font-display text-base sm:text-lg tracking-tight">{{ t('app.title') }}</div>
      <div class="text-[10px] uppercase tracking-[0.16em] text-primary/80">{{ t('app.subtitle') }}</div>
    </div>
  </div>
</template>
