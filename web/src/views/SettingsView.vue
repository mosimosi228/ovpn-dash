<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckIcon } from '@heroicons/vue/24/outline'
import { fetchSettings, patchSettings, type SetupState } from '@/api/client'
import { flash } from '@/lib/flash'
import FormField from '@/components/FormField.vue'

const emit = defineEmits<{ 'update:state': [Partial<SetupState>] }>()
const { t } = useI18n()

const form = reactive({
  pki_dir: '',
  server_conf: '',
  unit: '',
  log_file: '',
  public_host: '',
  smtp_host: '',
  smtp_port: '',
  smtp_user: '',
  smtp_pass: '',
  smtp_from: '',
  smtp_tls: true,
  telegram_bot_token: '',
})
const busy = ref(false)
const tokenSet = ref(false)

async function fill() {
  const s = await fetchSettings()
  form.pki_dir = s.pki_dir || ''
  form.server_conf = s.server_conf || ''
  form.unit = s.unit || ''
  form.log_file = s.log_file || ''
  form.public_host = s.public_host || ''
  form.smtp_host = s.smtp_host || ''
  form.smtp_port = s.smtp_port || ''
  form.smtp_user = s.smtp_user || ''
  form.smtp_from = s.smtp_from || ''
  form.smtp_tls = s.smtp_tls !== false
  tokenSet.value = !!s.telegram_token_set
  emit('update:state', s)
}

onMounted(() => {
  fill().catch(() => {})
})

async function save() {
  busy.value = true
  try {
    const body: Record<string, string | boolean> = {
      pki_dir: form.pki_dir,
      server_conf: form.server_conf,
      unit: form.unit,
      log_file: form.log_file,
      public_host: form.public_host,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_user: form.smtp_user,
      smtp_from: form.smtp_from,
      smtp_tls: form.smtp_tls,
    }
    if (form.smtp_pass) body.smtp_pass = form.smtp_pass
    if (form.telegram_bot_token) body.telegram_bot_token = form.telegram_bot_token
    const next = await patchSettings(body)
    emit('update:state', next)
    form.smtp_pass = ''
    form.telegram_bot_token = ''
    flash('success', t('settings.saved'))
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="font-display text-2xl mb-2">{{ t('settings.title') }}</h2>
    <p class="text-sm text-base-content/60 mb-6">{{ t('settings.lead') }}</p>
    <form class="form-stack" @submit.prevent="save">
      <FormField :label="t('wizard.pki')">
        <input v-model="form.pki_dir" class="input-field font-mono text-sm" required />
      </FormField>
      <FormField :label="t('wizard.conf')">
        <input v-model="form.server_conf" class="input-field font-mono text-sm" required />
      </FormField>
      <FormField :label="t('wizard.unit')">
        <input v-model="form.unit" class="input-field font-mono text-sm" required />
      </FormField>
      <FormField :label="t('wizard.log')">
        <input v-model="form.log_file" class="input-field font-mono text-sm" />
      </FormField>
      <FormField :label="t('wizard.host')">
        <input v-model="form.public_host" class="input-field" required />
      </FormField>

      <h3 class="font-display text-lg mt-2">{{ t('wizard.smtp') }}</h3>
      <FormField :label="t('wizard.smtpHost')">
        <input v-model="form.smtp_host" class="input-field font-mono text-sm" />
      </FormField>
      <FormField :label="t('wizard.smtpPort')">
        <input v-model="form.smtp_port" class="input-field font-mono text-sm" />
      </FormField>
      <FormField :label="t('wizard.smtpUser')">
        <input v-model="form.smtp_user" class="input-field" />
      </FormField>
      <FormField :label="t('wizard.smtpPass')">
        <input v-model="form.smtp_pass" class="input-field" type="password" />
      </FormField>
      <FormField :label="t('wizard.smtpFrom')">
        <input v-model="form.smtp_from" class="input-field" />
      </FormField>
      <FormField label="TLS">
        <label class="flex items-center gap-2 h-9 cursor-pointer">
          <input v-model="form.smtp_tls" type="checkbox" class="checkbox checkbox-sm" />
        </label>
      </FormField>

      <h3 class="font-display text-lg mt-2">{{ t('wizard.telegram') }}</h3>
      <FormField :label="t('wizard.telegramToken')">
        <input
          v-model="form.telegram_bot_token"
          class="input-field font-mono text-sm"
          :placeholder="tokenSet ? '••••' : ''"
        />
      </FormField>

      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          <CheckIcon class="size-4" />
          {{ t('settings.save') }}
        </button>
      </div>
    </form>
  </section>
</template>
