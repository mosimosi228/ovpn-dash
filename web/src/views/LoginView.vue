<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRightEndOnRectangleIcon, PaperAirplaneIcon } from '@heroicons/vue/24/outline'
import { forgotPassword, login, resetPassword, sendPIN, verifyPIN, type SetupState } from '@/api/client'
import FormField from '@/components/FormField.vue'

const props = defineProps<{ state: SetupState }>()
const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()

const email = ref('')
const password = ref('')
const pin = ref('')
const busy = ref(false)
const pinUntil = ref(0)
const now = ref(Date.now())
const showForgot = ref(false)
const forgotSent = ref(false)
const resetTok = ref('')
const resetPw = ref('')
const resetDone = ref(false)

let timer: number | undefined

onMounted(() => {
  const q = new URLSearchParams(window.location.search).get('reset') || ''
  resetTok.value = q
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 500)
})
onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

const pinLeft = () => Math.max(0, Math.ceil((pinUntil.value - now.value) / 1000))

async function submit() {
  busy.value = true
  try {
    await login(email.value, password.value)
    emit('done')
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onSendPIN() {
  busy.value = true
  try {
    const r = await sendPIN(email.value)
    pinUntil.value = (r.expires_at || 0) * 1000
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onVerifyPIN() {
  busy.value = true
  try {
    await verifyPIN(email.value, pin.value)
    emit('done')
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onForgot() {
  busy.value = true
  try {
    await forgotPassword(email.value)
    forgotSent.value = true
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onReset() {
  busy.value = true
  try {
    await resetPassword(resetTok.value, resetPw.value)
    resetDone.value = true
    resetTok.value = ''
    history.replaceState({}, '', window.location.pathname)
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel w-full max-w-lg">
    <h1 class="font-display text-3xl mb-6">
      {{ resetTok ? t('login.resetTitle') : t('login.title') }}
    </h1>

    <form v-if="resetTok" class="form-stack" @submit.prevent="onReset">
      <FormField :label="t('login.password')">
        <input v-model="resetPw" class="input-field" type="password" minlength="8" required />
      </FormField>
      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          {{ t('login.resetSubmit') }}
        </button>
      </div>
    </form>

    <p v-else-if="resetDone" class="text-sm text-success mb-4">{{ t('login.resetDone') }}</p>

    <form v-if="!resetTok" class="form-stack" @submit.prevent="submit">
      <FormField :label="t('login.email')">
        <input
          v-model="email"
          class="input-field"
          type="text"
          required
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
        />
      </FormField>
      <FormField :label="t('login.password')">
        <input v-model="password" class="input-field" type="password" required autocomplete="current-password" />
      </FormField>
      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          <ArrowRightEndOnRectangleIcon class="size-4" />
          {{ t('login.submit') }}
        </button>
      </div>
    </form>

    <template v-if="!resetTok && props.state.telegram_configured">
      <div class="divider text-xs">{{ t('login.or') }}</div>
      <button class="btn btn-ghost btn-sm gap-1.5" type="button" :disabled="busy || !email" @click="onSendPIN">
        <PaperAirplaneIcon class="size-4" />
        {{ t('login.pin') }}
      </button>
      <p v-if="pinUntil" class="text-xs text-base-content/60 mt-2">
        {{ t('login.pinSent') }}
        <span v-if="pinLeft() > 0">{{ t('login.pinTimer', { sec: pinLeft() }) }}</span>
      </p>
      <form v-if="pinUntil" class="form-stack mt-3" @submit.prevent="onVerifyPIN">
        <FormField :label="t('login.pinPlaceholder')">
          <input v-model="pin" class="input-field" maxlength="6" />
        </FormField>
        <div class="form-actions">
          <button class="btn btn-primary btn-sm" type="submit" :disabled="busy">{{ t('login.pinVerify') }}</button>
        </div>
      </form>
    </template>

    <template v-if="!resetTok && props.state.smtp_configured">
      <button class="btn btn-link btn-sm mt-3 px-0" type="button" @click="showForgot = !showForgot">
        {{ t('login.forgot') }}
      </button>
      <form v-if="showForgot" class="mt-2" @submit.prevent="onForgot">
        <button class="btn btn-ghost btn-sm w-fit" type="submit" :disabled="busy || !email">
          {{ t('login.forgotSubmit') }}
        </button>
        <p v-if="forgotSent" class="text-xs text-base-content/60 mt-2">{{ t('login.forgotSent') }}</p>
      </form>
    </template>
  </section>
</template>
