<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowDownTrayIcon } from '@heroicons/vue/24/outline'
import { downloadMyOvpn, type Me } from '@/api/client'

const props = defineProps<{ me: Me }>()
const { t } = useI18n()
</script>

<template>
  <section class="panel max-w-lg">
    <h2 class="font-display text-2xl mb-2">{{ t('home.title') }}</h2>
    <p class="text-sm text-base-content/60 mb-6">{{ t('home.lead') }}</p>
    <template v-if="props.me.client_name">
      <p class="text-sm mb-4">
        <span class="text-base-content/50">{{ t('clients.name') }}:</span>
        <span class="font-mono ml-2">{{ props.me.client_name }}</span>
      </p>
      <button class="btn btn-primary btn-sm gap-1.5" type="button" @click="downloadMyOvpn()">
        <ArrowDownTrayIcon class="size-4" />
        {{ t('home.download') }}
      </button>
    </template>
    <p v-else class="text-sm text-base-content/50">{{ t('home.noCert') }}</p>
  </section>
</template>
