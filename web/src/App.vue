<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchState, getAccessToken, setupToken, type SetupState } from '@/api/client'
import AppHeader from '@/components/AppHeader.vue'
import GridPattern from '@/components/GridPattern.vue'
import WizardView from '@/views/WizardView.vue'
import LoginView from '@/views/LoginView.vue'
import ConsoleView from '@/views/ConsoleView.vue'

const { t } = useI18n()
const state = ref<SetupState | null>(null)
const bootError = ref('')
const loading = ref(true)
const authed = ref(!!getAccessToken())

onMounted(async () => {
  try {
    state.value = await fetchState()
  } catch (e: unknown) {
    const status =
      e && typeof e === 'object' && 'response' in e
        ? (e as { response?: { status?: number } }).response?.status
        : 0
    if (status === 403) {
      bootError.value = setupToken ? t('boot.forbiddenToken') : t('boot.forbidden')
    } else {
      bootError.value = t('boot.failed')
    }
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="relative min-h-full flex flex-col">
    <GridPattern />
    <div class="relative z-10 min-h-full flex flex-col">
      <AppHeader />
      <main class="flex-1 w-full max-w-6xl mx-auto px-4 sm:px-6 py-8 sm:py-10">
        <div v-if="loading" class="text-sm text-base-content/60 font-mono">{{ t('boot.loading') }}</div>
        <div v-else-if="bootError" class="rounded-box border border-error/30 bg-error/5 p-6">
          <h2 class="font-display text-xl mb-2">{{ t('app.title') }}</h2>
          <p class="text-sm text-base-content/70">{{ bootError }}</p>
        </div>
        <WizardView v-else-if="state && !state.complete" :state="state" @update:state="state = $event" />
        <LoginView v-else-if="state && !authed" @done="authed = true" />
        <ConsoleView v-else-if="state" :state="state" />
      </main>
    </div>
  </div>
</template>
