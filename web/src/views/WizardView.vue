<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckIcon } from '@heroicons/vue/24/outline'
import { postSetup, type SetupState } from '@/api/client'
import FormField from '@/components/FormField.vue'

const props = defineProps<{ state: SetupState }>()
const emit = defineEmits<{ 'update:state': [SetupState] }>()
const { t } = useI18n()

const form = reactive({
  email: '',
  name: '',
  password: '',
  pki_dir: props.state.pki_dir || '/etc/openvpn/easy-rsa/pki',
  server_conf: props.state.server_conf || '/etc/openvpn/server/server.conf',
  unit: props.state.unit || 'openvpn-server@server',
  log_file: props.state.log_file || '/var/log/openvpn/server.log',
  public_host: '',
  smtp_host: '',
  smtp_port: '587',
  smtp_user: '',
  smtp_pass: '',
  smtp_from: '',
  telegram_bot_token: '',
})
const busy = ref(false)

async function submit() {
  busy.value = true
  try {
    const next = await postSetup({ ...form })
    emit('update:state', next)
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel w-full max-w-2xl">
    <h1 class="font-display text-3xl mb-2">{{ t('wizard.title') }}</h1>
    <p class="text-sm text-base-content/60 mb-6">{{ t('wizard.lead') }}</p>
    <form class="form-stack" @submit.prevent="submit">
      <FormField :label="t('wizard.email')">
        <input v-model="form.email" class="input-field" type="email" required />
      </FormField>
      <FormField :label="t('wizard.name')">
        <input v-model="form.name" class="input-field" />
      </FormField>
      <FormField :label="t('wizard.password')">
        <input v-model="form.password" class="input-field" type="password" minlength="8" required />
      </FormField>
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
        <input v-model="form.public_host" class="input-field" required placeholder="vpn.example.com" />
      </FormField>

      <h2 class="font-display text-lg mt-2">{{ t('wizard.smtp') }}</h2>
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
        <input v-model="form.smtp_from" class="input-field" type="email" />
      </FormField>

      <h2 class="font-display text-lg mt-2">{{ t('wizard.telegram') }}</h2>
      <FormField :label="t('wizard.telegramToken')">
        <input v-model="form.telegram_bot_token" class="input-field font-mono text-sm" />
      </FormField>

      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          <CheckIcon class="size-4" />
          {{ t('wizard.submit') }}
        </button>
      </div>
    </form>
  </section>
</template>
