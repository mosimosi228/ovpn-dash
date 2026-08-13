<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { login } from '@/api/client'

const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
const username = ref('admin')
const password = ref('')
const err = ref('')
const busy = ref(false)

async function submit() {
  err.value = ''
  busy.value = true
  try {
    await login(username.value, password.value)
    emit('done')
  } catch {
    err.value = t('login.error')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="rounded-box border border-base-content/10 bg-base-200/40 backdrop-blur-xl p-6 sm:p-8 max-w-md">
    <h1 class="font-display text-3xl mb-6">{{ t('login.title') }}</h1>
    <form class="flex flex-col gap-3" @submit.prevent="submit">
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('login.username') }}</label>
      <input v-model="username" class="input-field" required />
      <label class="text-xs uppercase tracking-wide text-base-content/50">{{ t('login.password') }}</label>
      <input v-model="password" class="input-field" type="password" required />
      <p v-if="err" class="text-error text-sm">{{ err }}</p>
      <button class="btn btn-primary mt-2" type="submit" :disabled="busy">{{ t('login.submit') }}</button>
    </form>
  </section>
</template>
