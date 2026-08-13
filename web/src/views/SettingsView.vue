<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { patchSettings, type SetupState } from '@/api/client'
import { flash } from '@/lib/flash'

const props = defineProps<{ state: SetupState }>()
const emit = defineEmits<{ 'update:state': [Partial<SetupState>] }>()
const { t } = useI18n()

const form = reactive({
  pki_dir: '',
  server_conf: '',
  unit: '',
  log_file: '',
  public_host: '',
})
const currentPassword = ref('')
const newPassword = ref('')
const busy = ref(false)

function fill() {
  form.pki_dir = props.state.pki_dir || ''
  form.server_conf = props.state.server_conf || ''
  form.unit = props.state.unit || ''
  form.log_file = props.state.log_file || ''
  form.public_host = props.state.public_host || ''
}

watch(() => props.state, fill, { immediate: true, deep: true })

async function save() {
  busy.value = true
  try {
    const body: Record<string, string> = { ...form }
    if (newPassword.value) {
      body.current_password = currentPassword.value
      body.password = newPassword.value
    }
    const next = await patchSettings(body)
    emit('update:state', next)
    currentPassword.value = ''
    newPassword.value = ''
    flash('success', t('settings.saved'))
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="rounded-box border border-base-content/10 bg-base-200/40 p-6">
    <h2 class="font-display text-2xl mb-2">{{ t('settings.title') }}</h2>
    <p class="text-sm text-base-content/60 mb-6">{{ t('settings.lead') }}</p>
    <form class="flex flex-col gap-3 max-w-xl" @submit.prevent="save">
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.pki') }}</label>
      <input v-model="form.pki_dir" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.conf') }}</label>
      <input v-model="form.server_conf" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.unit') }}</label>
      <input v-model="form.unit" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.log') }}</label>
      <input v-model="form.log_file" class="input-field font-mono text-sm" />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.host') }}</label>
      <input v-model="form.public_host" class="input-field" required />

      <h3 class="font-display text-lg mt-4">{{ t('settings.password') }}</h3>
      <p class="text-xs text-base-content/50 -mt-1">{{ t('settings.passwordHint') }}</p>
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('me.current') }}</label>
      <input v-model="currentPassword" class="input-field" type="password" autocomplete="current-password" />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('me.password') }}</label>
      <input v-model="newPassword" class="input-field" type="password" minlength="8" autocomplete="new-password" />

      <button class="btn btn-primary btn-sm w-fit mt-2" type="submit" :disabled="busy">{{ t('settings.save') }}</button>
    </form>
  </section>
</template>
