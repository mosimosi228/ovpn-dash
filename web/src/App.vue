<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchMe, fetchState, getAccessToken, setupToken, type Me, type SetupState } from '@/api/client'
import { applyTheme } from '@/lib/theme'
import AppHeader from '@/components/AppHeader.vue'
import AppFooter from '@/components/AppFooter.vue'
import FlashHost from '@/components/FlashHost.vue'
import GridPattern from '@/components/GridPattern.vue'
import WizardView from '@/views/WizardView.vue'
import LoginView from '@/views/LoginView.vue'
import ConsoleView from '@/views/ConsoleView.vue'

const { t } = useI18n()
const state = ref<SetupState | null>(null)
const me = ref<Me | null>(null)
const bootError = ref('')
const loading = ref(true)
const authed = ref(!!getAccessToken())

const showConsole = computed(
  () => !loading.value && !bootError.value && !!state.value?.complete && authed.value && !!me.value,
)

async function loadMe() {
  me.value = await fetchMe()
  applyTheme(me.value.theme || 'light')
}

onMounted(async () => {
  try {
    state.value = await fetchState()
    if (authed.value && state.value?.complete) {
      try {
        await loadMe()
      } catch {
        authed.value = false
      }
    }
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

async function onLogin() {
  authed.value = true
  try {
    await loadMe()
  } catch {
    authed.value = false
  }
}
</script>

<template>
  <div class="relative">
    <GridPattern />
    <div class="relative z-10 pt-14">
      <AppHeader v-if="!showConsole" />
      <FlashHost />
      <main class="page-shell">
        <div v-if="loading" class="text-sm text-base-content/60 font-mono">{{ t('boot.loading') }}</div>
        <div v-else-if="bootError" class="panel max-w-lg">
          <h2 class="font-display text-xl mb-2">{{ t('app.title') }}</h2>
          <p class="text-sm text-base-content/70">{{ bootError }}</p>
        </div>
        <WizardView v-else-if="state && !state.complete" :state="state" @update:state="state = $event" />
        <LoginView v-else-if="state && !authed" :state="state" @done="onLogin" />
        <ConsoleView
          v-else-if="state && me"
          :state="state"
          :me="me"
          @update:state="Object.assign(state, $event)"
          @update:me="me = $event"
        />
      </main>
      <AppFooter />
    </div>
  </div>
</template>
