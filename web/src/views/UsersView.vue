<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { NoSymbolIcon, PlusIcon, TrashIcon } from '@heroicons/vue/24/outline'
import { createUser, deleteUser, fetchUsers, patchUser, type DashUser, type Me } from '@/api/client'
import FormField from '@/components/FormField.vue'

const props = defineProps<{ me: Me }>()
const { t } = useI18n()
const items = ref<DashUser[]>([])
const busy = ref(false)
const form = reactive({
  email: '',
  name: '',
  password: '',
  client_name: '',
  role: 'user',
})

async function load() {
  items.value = await fetchUsers()
}

onMounted(() => {
  load().catch(() => {})
})

async function onCreate() {
  busy.value = true
  try {
    const body: Record<string, string> = {
      email: form.email,
      name: form.name,
      password: form.password,
      role: form.role,
    }
    if (form.role === 'user') body.client_name = form.client_name
    await createUser(body)
    form.email = ''
    form.name = ''
    form.password = ''
    form.client_name = ''
    await load()
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}

async function onRole(u: DashUser, ev: Event) {
  const role = (ev.target as HTMLSelectElement).value
  if (role === u.role) return
  try {
    await patchUser(u.id, { role })
    await load()
  } catch {
    await load()
  }
}

async function onToggle(u: DashUser) {
  try {
    await patchUser(u.id, { disabled: !u.disabled })
    await load()
  } catch {
    /* flashed */
  }
}

async function onDelete(u: DashUser) {
  if (!confirm(t('users.confirmDelete', { email: u.email }))) return
  try {
    await deleteUser(u.id)
    await load()
  } catch {
    /* flashed */
  }
}
</script>

<template>
  <div class="contents">
  <section class="panel">
    <h2 class="font-display text-2xl mb-5">{{ t('users.create') }}</h2>
    <form class="form-stack" @submit.prevent="onCreate">
      <FormField :label="t('profile.email')">
        <input v-model="form.email" class="input-field" type="email" :required="form.role !== 'user'" />
      </FormField>
      <FormField :label="t('profile.name')">
        <input v-model="form.name" class="input-field" />
      </FormField>
      <FormField :label="t('clients.password')">
        <input v-model="form.password" class="input-field" type="password" minlength="8" required />
      </FormField>
      <FormField v-if="form.role === 'user'" :label="t('clients.name')">
        <input v-model="form.client_name" class="input-field font-mono" :placeholder="t('clients.placeholder')" />
      </FormField>
      <FormField :label="t('users.role')">
        <select v-model="form.role" class="input-field">
          <option value="user">{{ t('users.user') }}</option>
          <option v-if="props.me.role === 'root'" value="admin">{{ t('users.admin') }}</option>
        </select>
      </FormField>
      <div class="form-actions">
        <button class="btn btn-primary btn-sm gap-1.5" type="submit" :disabled="busy">
          <PlusIcon class="size-4" />
          {{ form.role === 'admin' ? t('users.addAdmin') : t('users.addUser') }}
        </button>
      </div>
    </form>
  </section>

  <section class="panel">
    <h2 class="font-display text-2xl mb-5">{{ t('users.list') }}</h2>
    <p v-if="!items.length" class="text-base-content/50 text-sm">{{ t('users.empty') }}</p>
    <div v-else class="overflow-x-auto">
      <table class="table table-sm">
        <thead>
          <tr>
            <th>{{ t('profile.email') }}</th>
            <th>{{ t('profile.name') }}</th>
            <th>{{ t('users.role') }}</th>
            <th>{{ t('clients.name') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in items" :key="u.id">
            <td class="font-mono text-xs">
              {{ u.email }}
              <span v-if="u.disabled" class="badge badge-error badge-xs ml-2">{{ t('clients.disabled') }}</span>
            </td>
            <td>{{ u.name }}</td>
            <td>
              <select
                v-if="props.me.role === 'root' && u.role !== 'root'"
                class="select select-xs select-bordered"
                :value="u.role"
                @change="onRole(u, $event)"
              >
                <option value="user">{{ t('users.user') }}</option>
                <option value="admin">{{ t('users.admin') }}</option>
              </select>
              <span v-else>{{ u.role }}</span>
            </td>
            <td class="font-mono text-xs">{{ u.client_name }}</td>
            <td class="text-right whitespace-nowrap">
              <button
                v-if="u.role !== 'root' && (props.me.role === 'root' || u.role === 'user')"
                class="btn btn-ghost btn-xs gap-1"
                type="button"
                @click="onToggle(u)"
              >
                <NoSymbolIcon class="size-3.5" />
                {{ u.disabled ? t('users.enable') : t('users.disable') }}
              </button>
              <button
                v-if="u.role !== 'root' && (props.me.role === 'root' || u.role === 'user')"
                class="btn btn-ghost btn-xs gap-1 text-error"
                type="button"
                @click="onDelete(u)"
              >
                <TrashIcon class="size-3.5" />
                {{ t('users.delete') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
  </div>
</template>
