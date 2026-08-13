<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  clearTokens,
  createClient,
  downloadOvpn,
  fetchClients,
  fetchConnections,
  fetchLog,
  fetchServer,
  revokeClient,
  startServer,
  stopServer,
  type Client,
  type Connection,
  type ServerStatus,
  type SetupState,
} from '@/api/client'
import MapView from '@/views/MapView.vue'
import SettingsView from '@/views/SettingsView.vue'

const props = defineProps<{ state: SetupState }>()
const emit = defineEmits<{ 'update:state': [Partial<SetupState>] }>()
const { t } = useI18n()

const tab = ref<'server' | 'clients' | 'map' | 'log' | 'settings'>('server')
const server = ref<ServerStatus | null>(null)
const clients = ref<Client[]>([])
const connections = ref<Connection[]>([])
const mapHint = ref('')
const newName = ref('')
const logText = ref('')
const err = ref('')
const busy = ref(false)

async function loadServer() {
  server.value = await fetchServer()
}
async function loadClients() {
  clients.value = await fetchClients()
}
async function loadConnections() {
  try {
    const data = await fetchConnections()
    connections.value = data.items || []
    mapHint.value = data.hint || ''
  } catch {
    connections.value = []
  }
}
async function loadLog() {
  try {
    const l = await fetchLog()
    logText.value = l.text
  } catch {
    logText.value = ''
  }
}

onMounted(async () => {
  try {
    await loadServer()
    await loadClients()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : 'error'
  }
})

async function onStart() {
  busy.value = true
  try {
    server.value = await startServer()
  } finally {
    busy.value = false
  }
}
async function onStop() {
  busy.value = true
  try {
    server.value = await stopServer()
  } finally {
    busy.value = false
  }
}
async function onCreate() {
  if (!newName.value.trim()) return
  await createClient(newName.value.trim())
  newName.value = ''
  await loadClients()
}
async function onRevoke(name: string) {
  if (!confirm(t('clients.confirm', { name }))) return
  await revokeClient(name)
  await loadClients()
}
function fmtBytes(n: number) {
  if (!n) return '0'
  const u = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v < 10 && i ? v.toFixed(1) : Math.round(v)} ${u[i]}`
}

function logout() {
  clearTokens()
  location.reload()
}

function onSettingsSaved(next: Partial<SetupState>) {
  emit('update:state', next)
  loadServer().catch(() => {})
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-center gap-2">
      <button class="btn btn-sm" :class="tab === 'server' ? 'btn-primary' : 'btn-ghost'" @click="tab = 'server'">
        {{ t('nav.server') }}
      </button>
      <button class="btn btn-sm" :class="tab === 'clients' ? 'btn-primary' : 'btn-ghost'" @click="tab = 'clients'">
        {{ t('nav.clients') }}
      </button>
      <button
        class="btn btn-sm"
        :class="tab === 'map' ? 'btn-primary' : 'btn-ghost'"
        @click="
          tab = 'map';
          loadConnections()
        "
      >
        {{ t('nav.map') }}
      </button>
      <button
        class="btn btn-sm"
        :class="tab === 'log' ? 'btn-primary' : 'btn-ghost'"
        @click="
          tab = 'log';
          loadLog()
        "
      >
        {{ t('nav.log') }}
      </button>
      <button class="btn btn-sm" :class="tab === 'settings' ? 'btn-primary' : 'btn-ghost'" @click="tab = 'settings'">
        {{ t('nav.settings') }}
      </button>
      <button class="btn btn-ghost btn-sm ml-auto" type="button" @click="logout">{{ t('nav.logout') }}</button>
    </div>

    <p v-if="err" class="text-error text-sm">{{ err }}</p>

    <section v-if="tab === 'server'" class="rounded-box border border-base-content/10 bg-base-200/40 p-6">
      <div class="flex items-center gap-3 mb-4">
        <h2 class="font-display text-2xl">{{ t('server.title') }}</h2>
        <span class="badge" :class="server?.active ? 'badge-success' : 'badge-ghost'">
          {{ server?.active ? t('server.active') : t('server.inactive') }}
        </span>
      </div>
      <dl class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm font-mono mb-4">
        <div><dt class="text-base-content/50">{{ t('server.unit') }}</dt><dd>{{ server?.unit || props.state.unit }}</dd></div>
        <div><dt class="text-base-content/50">{{ t('server.proto') }}</dt><dd>{{ server?.proto }} {{ server?.port }}</dd></div>
        <div><dt class="text-base-content/50">{{ t('server.host') }}</dt><dd>{{ server?.public_host }}</dd></div>
        <div><dt class="text-base-content/50">cipher</dt><dd>{{ server?.cipher }}</dd></div>
      </dl>
      <div v-if="server?.warnings?.length" class="alert alert-warning text-sm mb-4">
        <div>
          <div class="font-semibold mb-1">{{ t('server.warnings') }}</div>
          <p v-if="server.warnings.includes('crl-verify')">{{ t('server.warnCrl') }}</p>
          <p v-if="server.warnings.includes('tls-crypt')">{{ t('server.warnTls') }}</p>
        </div>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-success btn-sm" :disabled="busy || server?.active" @click="onStart">{{ t('server.start') }}</button>
        <button class="btn btn-error btn-sm" :disabled="busy || !server?.active" @click="onStop">{{ t('server.stop') }}</button>
      </div>
    </section>

    <section v-if="tab === 'clients'" class="rounded-box border border-base-content/10 bg-base-200/40 p-6">
      <h2 class="font-display text-2xl mb-4">{{ t('clients.title') }}</h2>
      <form class="flex gap-2 mb-4" @submit.prevent="onCreate">
        <input v-model="newName" class="input-field max-w-xs" :placeholder="t('clients.placeholder')" />
        <button class="btn btn-primary btn-sm" type="submit">{{ t('clients.add') }}</button>
      </form>
      <p v-if="!clients.length" class="text-base-content/50 text-sm">{{ t('clients.empty') }}</p>
      <table v-else class="table table-sm">
        <thead>
          <tr>
            <th>{{ t('clients.name') }}</th>
            <th>{{ t('clients.expires') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in clients" :key="c.name">
            <td class="font-mono">
              {{ c.name }}
              <span v-if="c.revoked" class="badge badge-error badge-xs ml-2">{{ t('clients.revoked') }}</span>
            </td>
            <td class="font-mono text-xs">{{ c.not_after?.slice(0, 10) }}</td>
            <td class="text-right">
              <button class="btn btn-ghost btn-xs" :disabled="c.revoked || !c.has_key" @click="downloadOvpn(c.name)">
                {{ t('clients.download') }}
              </button>
              <button class="btn btn-ghost btn-xs text-error" :disabled="c.revoked" @click="onRevoke(c.name)">
                {{ t('clients.revoke') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-if="tab === 'map'" class="rounded-box border border-base-content/10 bg-base-200/40 p-6">
      <div class="flex items-center mb-4">
        <h2 class="font-display text-2xl">{{ t('map.title') }}</h2>
        <button class="btn btn-ghost btn-sm ml-auto" @click="loadConnections">{{ t('map.refresh') }}</button>
      </div>
      <p v-if="mapHint === 'status'" class="text-warning text-sm mb-4">{{ t('map.hint') }}</p>
      <MapView :items="connections" />
      <p v-if="!connections.length && mapHint !== 'status'" class="text-base-content/50 text-sm mt-4">{{ t('map.empty') }}</p>
      <table v-if="connections.length" class="table table-sm mt-4">
        <thead>
          <tr>
            <th>{{ t('clients.name') }}</th>
            <th>{{ t('map.ip') }}</th>
            <th>{{ t('map.vpn') }}</th>
            <th>{{ t('map.since') }}</th>
            <th>{{ t('map.traffic') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in connections" :key="c.name + c.real_address">
            <td class="font-mono">{{ c.name }}</td>
            <td class="font-mono text-xs">
              {{ c.real_ip }}
              <span v-if="c.city || c.country" class="text-base-content/50"> {{ [c.city, c.country].filter(Boolean).join(', ') }}</span>
            </td>
            <td class="font-mono text-xs">{{ c.virtual_ip }}</td>
            <td class="font-mono text-xs">{{ c.since }}</td>
            <td class="font-mono text-xs">↓{{ fmtBytes(c.bytes_received) }} ↑{{ fmtBytes(c.bytes_sent) }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <SettingsView v-if="tab === 'settings'" :state="props.state" @update:state="onSettingsSaved" />

    <section v-if="tab === 'log'" class="rounded-box border border-base-content/10 bg-base-200/40 p-6">
      <div class="flex items-center mb-4">
        <h2 class="font-display text-2xl">{{ t('log.title') }}</h2>
        <button class="btn btn-ghost btn-sm ml-auto" @click="loadLog">{{ t('log.refresh') }}</button>
      </div>
      <pre v-if="logText" class="font-mono text-xs whitespace-pre-wrap bg-base-300/60 p-4 rounded-box max-h-[28rem] overflow-auto">{{ logText }}</pre>
      <p v-else class="text-base-content/50 text-sm">{{ t('log.empty') }}</p>
    </section>
  </div>
</template>
