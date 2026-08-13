<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { postSetup, type SetupState } from '@/api/client'

const props = defineProps<{ state: SetupState }>()
const emit = defineEmits<{ 'update:state': [SetupState] }>()
const { t } = useI18n()

const form = reactive({
  username: 'admin',
  password: '',
  pki_dir: '/etc/openvpn/easy-rsa/pki',
  server_conf: '/etc/openvpn/server/server.conf',
  unit: 'openvpn-server@server',
  log_file: '/var/log/openvpn/server.log',
  public_host: '',
})
const err = ref('')
const busy = ref(false)

async function submit() {
  err.value = ''
  busy.value = true
  try {
    const next = await postSetup({ ...form })
    emit('update:state', next)
  } catch (e: unknown) {
    const msg =
      e && typeof e === 'object' && 'response' in e
        ? (e as { response?: { data?: { error?: string } } }).response?.data?.error
        : ''
    err.value = msg || t('wizard.error')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="rounded-box border border-base-content/10 bg-base-200/40 backdrop-blur-xl p-6 sm:p-8 max-w-xl">
    <h1 class="font-display text-3xl mb-2">{{ t('wizard.title') }}</h1>
    <p class="text-sm text-base-content/60 mb-6">{{ t('wizard.lead') }}</p>
    <form class="flex flex-col gap-3" @submit.prevent="submit">
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.username') }}</label>
      <input v-model="form.username" class="input-field" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.password') }}</label>
      <input v-model="form.password" class="input-field" type="password" minlength="8" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.pki') }}</label>
      <input v-model="form.pki_dir" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.conf') }}</label>
      <input v-model="form.server_conf" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.unit') }}</label>
      <input v-model="form.unit" class="input-field font-mono text-sm" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.log') }}</label>
      <input v-model="form.log_file" class="input-field font-mono text-sm" />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('wizard.host') }}</label>
      <input v-model="form.public_host" class="input-field" required placeholder="vpn.example.com" />
      <p v-if="err" class="text-error text-sm">{{ err }}</p>
      <button class="btn btn-primary mt-2" type="submit" :disabled="busy">{{ t('wizard.submit') }}</button>
    </form>
    <p v-if="props.state.has_admin" class="hidden">{{ props.state.admin_user }}</p>
  </section>
</template>
