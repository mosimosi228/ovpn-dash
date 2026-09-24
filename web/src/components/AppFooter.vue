<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchVersion } from '@/api/client'

const { t } = useI18n()
const year = computed(() => new Date().getFullYear())
const favicon = `${import.meta.env.BASE_URL}favicon.svg`
const version = ref('')
const update = ref('')
const repo = ref('https://github.com/mosimosi228/ovpn-dash')

onMounted(() => {
  fetchVersion()
    .then((v) => {
      version.value = v.version || ''
      update.value = v.update || ''
      if (v.repo) repo.value = v.repo
    })
    .catch(() => {})
})
</script>

<template>
  <footer class="relative z-10 border-t border-primary/10 bg-base-300/30 backdrop-blur-xl">
    <div class="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
    <div class="max-w-6xl mx-auto px-4 sm:px-6 py-6 flex flex-col sm:flex-row gap-3 items-center justify-between">
      <div class="text-center sm:text-left">
        <div class="flex items-center justify-center sm:justify-start gap-2">
          <img :src="favicon" alt="" class="size-5 rounded" width="20" height="20" />
          <div class="font-display text-sm text-base-content">{{ t('app.title') }}</div>
        </div>
        <p class="text-xs text-base-content/50 mt-1 max-w-md">{{ t('footer.desc') }}</p>
        <a
          class="inline-flex items-center gap-1.5 text-xs text-primary/80 hover:text-primary mt-2"
          :href="repo"
          target="_blank"
          rel="noopener noreferrer"
        >
          <span class="size-1.5 rounded-full bg-primary/70" />
          {{ t('footer.repo') }}
        </a>
      </div>
      <div class="text-xs text-base-content/45 text-center sm:text-right">
        <div v-if="version">{{ t('footer.version', { version }) }}</div>
        <a
          v-if="update"
          class="mt-1 block text-primary/80 hover:text-primary"
          :href="repo"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ t('footer.update', { version: update }) }}
        </a>
        <div class="mt-1">© {{ year }}</div>
      </div>
    </div>
  </footer>
</template>
