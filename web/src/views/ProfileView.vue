<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckIcon, LinkIcon, LinkSlashIcon } from '@heroicons/vue/24/outline'
import { bindTelegram, patchMe, unbindTelegram, type Me } from '@/api/client'
import { applyTheme } from '@/lib/theme'
import { flash } from '@/lib/flash'
import FormField from '@/components/FormField.vue'

const props = defineProps<{ me: Me }>()
const emit = defineEmits<{ 'update:me': [Me] }>()
const { t } = useI18n()

const form = reactive({
  name: '',
  email: '',
  theme: 'light' as Me['theme'],
  current_password: '',
  password: '',
})
const busy = ref(false)
const bindCmd = ref('')
const bindBot = ref('')

function fill() {
  form.name = props.me.name
  form.email = props.me.email
  form.theme = props.me.theme || 'light'
}

watch(() => props.me, fill, { immediate: true, deep: true })

async function save() {
  busy.value = true
  try {
    const body: Record<string, string> = {
      name: form.name,
      email: form.email,
      theme: form.theme,
    }
    if (form.password) {
      body.current_password = form.current_password
      body.password = form.password
    }
    const next = await patchMe(body)
    applyTheme(next.theme)
    emit('update:me', next)
    form.password = ''
    form.current_password = ''
    flash('success', t('me.saved'))
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onBind() {
  try {
    const r = await bindTelegram()
    bindCmd.value = r.command
    bindBot.value = r.bot_username || props.me.telegram_bot_username || ''
  } catch {
    /* flashed */
  }
}

async function onUnbind() {
  try {
    const next = await unbindTelegram()
    emit('update:me', next)
    bindCmd.value = ''
  } catch {
    /* flashed */
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="font-display text-2xl mb-5">{{ t('profile.title') }}</h2>
    <form class="form-stack" @submit.prevent="save">
      <FormField :label="t('profile.name')">
        <input v-model="form.name" class="input-field" required />
      </FormField>
      <FormField :label="t('profile.email')">
        <input v-model="form.email" class="input-field" type="email" :required="props.me.role !== 'user'" />
      </FormField>
      <FormField :label="t('profile.theme')">
        <select v-model="form.theme" class="input-field">
          <option value="light">{{ t('profile.themeLight') }}</option>
          <option value="dark">{{ t('profile.themeDark') }}</option>
        </select>
      </FormField>
      <FormField :label="t('me.current')">
        <input v-model="form.current_password" class="input-field" type="password" autocomplete="current-password" />
      </FormField>
      <FormField :label="t('me.password')">
        <input v-model="form.password" class="input-field" type="password" minlength="8" autocomplete="new-password" />
      </FormField>
      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          <CheckIcon class="size-4" />
          {{ t('me.save') }}
        </button>
      </div>
    </form>

    <div class="mt-8">
      <h3 class="font-display text-lg mb-2">{{ t('profile.telegram') }}</h3>
      <p class="text-sm mb-3">
        {{ props.me.telegram_bound ? t('profile.telegramBound') : t('profile.telegramUnbound') }}
      </p>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-ghost btn-sm gap-1.5" type="button" @click="onBind">
          <LinkIcon class="size-4" />
          {{ t('profile.telegramBind') }}
        </button>
        <button v-if="props.me.telegram_bound" class="btn btn-ghost btn-sm gap-1.5" type="button" @click="onUnbind">
          <LinkSlashIcon class="size-4" />
          {{ t('profile.telegramUnbind') }}
        </button>
      </div>
      <p v-if="bindCmd" class="text-sm mt-2 font-mono">
        <a v-if="bindBot" class="link" :href="`https://t.me/${bindBot}`" target="_blank" rel="noreferrer">@{{ bindBot }}</a>
        {{ t('profile.telegramHint', { cmd: bindCmd }) }}
      </p>
    </div>
  </section>
</template>
