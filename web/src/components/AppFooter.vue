<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const year = computed(() => new Date().getFullYear())
</script>

<template>
  <footer class="relative mt-auto z-10 border-t border-primary/10 bg-base-300/30 backdrop-blur-xl">
    <div class="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
    <div class="max-w-6xl mx-auto px-4 sm:px-6 py-6 flex flex-col sm:flex-row gap-3 items-center justify-between">
      <div class="text-center sm:text-left">
        <div class="font-display text-sm text-base-content">{{ t('app.title') }}</div>
        <p class="text-xs text-base-content/50 mt-1 max-w-md">{{ t('footer.desc') }}</p>
        <a
          class="inline-block text-xs font-mono text-primary/80 hover:text-primary mt-2"
          href="https://github.com/mosimosi228/ovpn-dash"
          target="_blank"
          rel="noopener noreferrer"
        >
          github.com/mosimosi228/ovpn-dash
        </a>
      </div>
      <div class="text-xs text-base-content/45 font-mono text-center sm:text-right">
        <div>{{ t('footer.listen') }}</div>
        <div class="mt-1">© {{ year }}</div>
      </div>
    </div>
  </footer>
</template>
