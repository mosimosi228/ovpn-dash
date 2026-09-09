<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowDownTrayIcon,
  ArrowPathIcon,
  MagnifyingGlassIcon,
  NoSymbolIcon,
  PlayIcon,
  PlusIcon,
  StopIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'
import {
  clearTokens,
  createClient,
  downloadOvpn,
  fetchClients,
  fetchConnections,
  fetchLog,
  fetchServer,
  killConnection,
  openConnectionsSocket,
  reissueClient,
  revokeClient,
  startServer,
  stopServer,
  type Client,
  type Connection,
  type Me,
  type ServerStatus,
  type SetupState,
} from '@/api/client'
import AppNav from '@/components/AppNav.vue'
import FormField from '@/components/FormField.vue'
import MapView from '@/views/MapView.vue'
import SettingsView from '@/views/SettingsView.vue'
import ProfileView from '@/views/ProfileView.vue'
import UsersView from '@/views/UsersView.vue'
import HomeView from '@/views/HomeView.vue'
import { flash } from '@/lib/flash'
import { tabFromPath, writeTabUrl, type DashTab } from '@/lib/tabs'

const props = defineProps<{ state: SetupState; me: Me }>()
const emit = defineEmits<{ 'update:state': [Partial<SetupState>]; 'update:me': [Me] }>()
const { t } = useI18n()

const staff = computed(() => props.me.role === 'root' || props.me.role === 'admin')
const tab = ref<DashTab>(tabFromPath(staff.value))
const server = ref<ServerStatus | null>(null)
const clients = ref<Client[]>([])
const connections = ref<Connection[]>([])
const mapHint = ref('')
const mapStatusFile = ref('')
const canKill = ref(false)
let lastMapFlash = ''
const newName = ref('')
const newEmail = ref('')
const newPassword = ref('')
const clientQuery = ref('')
const showRevoked = ref(false)
const logText = ref('')
const logHint = ref('')
const logSource = ref('')
const busy = ref(false)
const nowTick = ref(Date.now())
let stopConnections: (() => void) | null = null
let tickTimer = 0

const protoPort = computed(() => {
  const proto = (server.value?.proto || 'udp').toUpperCase()
  const port = server.value?.port ?? 1194
  return `${proto} ${port}`
})

const filteredClients = computed(() => {
  const q = clientQuery.value.trim().toLowerCase()
  return clients.value.filter((c) => {
    if (!showRevoked.value && (c.revoked || c.disabled)) return false
    if (!q) return true
    const hay = `${c.name} ${c.email || ''}`.toLowerCase()
    return hay.includes(q)
  })
})

async function loadServer() {
  server.value = await fetchServer()
}
async function loadClients() {
  clients.value = await fetchClients()
}
async function loadConnections(forceFlash = false) {
  try {
    applyConnections(await fetchConnections(), forceFlash)
  } catch {
    connections.value = []
  }
}

function formatMapHint(hint: string, path: string) {
  if (!hint) return ''
  if (hint === 'status') return t('map.hint')
  if (hint === 'denied') return t('map.hintDenied', { path: path || '/var/log/openvpn/status.log' })
  if (hint === 'missing') return t('map.hintMissing', { path: path || '' })
  if (hint === 'server.conf unreadable') return t('map.hintConf')
  return hint
}

const mapHintText = computed(() => formatMapHint(mapHint.value, mapStatusFile.value))

function applyConnections(data: { items?: Connection[]; hint?: string; status_file?: string; can_kill?: boolean }, forceFlash = false) {
  connections.value = data.items || []
  mapHint.value = data.hint || ''
  mapStatusFile.value = data.status_file || ''
  canKill.value = !!data.can_kill
  const msg = formatMapHint(mapHint.value, mapStatusFile.value)
  if (!msg) {
    lastMapFlash = ''
    return
  }
  if (forceFlash || msg !== lastMapFlash) {
    lastMapFlash = msg
    flash('warning', msg)
  }
}
async function loadLog() {
  try {
    const l = await fetchLog()
    logText.value = l.text || ''
    logHint.value = l.hint || ''
    logSource.value = l.source || ''
  } catch (e: unknown) {
    logText.value = ''
    logSource.value = ''
    logHint.value =
      e && typeof e === 'object' && 'response' in e
        ? (e as { response?: { data?: { error?: string } } }).response?.data?.error || 'error'
        : 'error'
  }
}

onMounted(async () => {
  tab.value = tabFromPath(staff.value)
  writeTabUrl(tab.value, true)
  window.addEventListener('popstate', onPop)
  if (!staff.value) return
  try {
    await loadServer()
    await loadClients()
  } catch {
    /* flashed */
  }
  startLiveConnections()
})

onUnmounted(() => {
  window.removeEventListener('popstate', onPop)
  stopConnections?.()
  stopConnections = null
  if (tickTimer) window.clearInterval(tickTimer)
})

function onPop() {
  tab.value = tabFromPath(staff.value)
}

function setTab(next: DashTab) {
  if (tab.value === next) {
    writeTabUrl(next, true)
    return
  }
  tab.value = next
  writeTabUrl(next)
}

function startLiveConnections() {
  loadConnections()
  nowTick.value = Date.now()
  if (tickTimer) window.clearInterval(tickTimer)
  tickTimer = window.setInterval(() => {
    nowTick.value = Date.now()
  }, 1000)
  stopConnections?.()
  stopConnections = openConnectionsSocket((data) => {
    applyConnections(data)
  })
}

watch(tab, (next) => {
  if (next === 'clients' && staff.value) loadClients()
  if (next === 'log') loadLog()
})

async function onStart() {
  busy.value = true
  try {
    server.value = await startServer()
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
async function onStop() {
  busy.value = true
  try {
    server.value = await stopServer()
  } catch {
    /* flashed */
  } finally {
    busy.value = false
  }
}
async function onCreate() {
  if (!newEmail.value.trim() || !newPassword.value) return
  try {
    await createClient({
      email: newEmail.value.trim(),
      password: newPassword.value,
      name: newName.value.trim(),
      client_name: newName.value.trim(),
    })
    newName.value = ''
    newEmail.value = ''
    newPassword.value = ''
    await loadClients()
  } catch {
    /* flashed */
  }
}
async function onRevoke(name: string) {
  if (!confirm(t('clients.confirm', { name }))) return
  try {
    await revokeClient(name)
    await loadClients()
  } catch {
    /* flashed */
  }
}
async function onReissue(name: string) {
  if (!confirm(t('clients.confirmReissue', { name }))) return
  try {
    const r = await reissueClient(name)
    if (r.reload_error) flash('warning', r.reload_error)
    else flash('success', t('clients.reissued', { name }))
  } catch {
    /* flashed */
  }
  try {
    await loadClients()
  } catch {
    /* flashed */
  }
}
function fmtBytes(n: number) {
  if (!n) return '0'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v < 10 && i ? v.toFixed(1) : Math.round(v)} ${u[i]}`
}

function fmtTraffic(n: number) {
  return `${n} (${fmtBytes(n)})`
}

function flagEmoji(cc?: string) {
  if (!cc || cc.length !== 2) return ''
  const u = cc.toUpperCase()
  return String.fromCodePoint(127397 + u.charCodeAt(0), 127397 + u.charCodeAt(1))
}

function locationText(c: Connection) {
  const flag = flagEmoji(c.country_code)
  const bits = [c.city, c.region, c.country].filter(Boolean)
  const text = bits.join(', ')
  if (flag && text) return `${flag} ${text}`
  return text || flag || '—'
}

function timeOnline(c: Connection) {
  const ms = c.since_unix ? c.since_unix * 1000 : Date.parse(c.since || '')
  if (!ms || Number.isNaN(ms)) return '—'
  let sec = Math.max(0, Math.floor((nowTick.value - ms) / 1000))
  const days = Math.floor(sec / 86400)
  sec %= 86400
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  const clock = `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return days ? `${days}d ${clock}` : clock
}

function logout() {
  clearTokens()
  location.reload()
}

function onSettingsSaved(next: Partial<SetupState>) {
  emit('update:state', next)
  loadServer().catch(() => {})
}

function onMe(next: Me) {
  emit('update:me', next)
}

async function onDisconnect(c: Connection) {
  if (!canKill.value) {
    flash('warning', t('map.hintManage'))
    return
  }
  if (!confirm(t('map.confirmDisconnect', { name: c.name }))) return
  try {
    await killConnection({ name: c.name, real_address: c.real_address })
    flash('success', t('map.disconnected', { name: c.name }))
    await loadConnections()
  } catch {
    /* flashed */
  }
}
</script>

<template>
  <div class="w-full max-w-6xl flex flex-col gap-5">
    <AppNav :tab="tab" :staff="staff" @update:tab="setTab" @logout="logout" />
      <section v-if="tab === 'server' && staff" class="panel">
        <div class="flex flex-wrap items-start justify-between gap-4 mb-6">
          <div>
            <h2 class="font-display text-2xl">{{ t('server.title') }}</h2>
            <p class="text-xs font-mono text-base-content/45 mt-1">{{ server?.unit || props.state.unit }}</p>
          </div>
          <div class="flex gap-2">
            <button class="btn btn-success btn-sm gap-1.5" :disabled="busy || server?.active" @click="onStart">
              <PlayIcon class="size-4" />
              {{ t('server.start') }}
            </button>
            <button class="btn btn-error btn-sm gap-1.5" :disabled="busy || !server?.active" @click="onStop">
              <StopIcon class="size-4" />
              {{ t('server.stop') }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-16 gap-y-7 max-w-xl">
          <div>
            <div class="stat-label">{{ t('server.remote') }}</div>
            <div class="stat-value">{{ server?.public_host || '—' }}</div>
          </div>
          <div>
            <div class="stat-label">{{ t('server.protoPort') }}</div>
            <div class="stat-value">{{ protoPort }}</div>
          </div>
          <div>
            <div class="stat-label">{{ t('server.local') }}</div>
            <div class="stat-value">{{ server?.network || '—' }}</div>
          </div>
          <div>
            <div class="stat-label">{{ t('server.cipher') }}</div>
            <div class="stat-value">{{ server?.cipher || '—' }}</div>
          </div>
          <div>
            <div class="stat-label">{{ t('server.sessions') }}</div>
            <div class="stat-value">{{ connections.length }}</div>
          </div>
        </div>

        <div v-if="server?.warnings?.length" class="alert alert-warning text-sm mt-6">
          <div>
            <div class="font-semibold mb-1">{{ t('server.warnings') }}</div>
            <p v-if="server.warnings.includes('crl-verify')">{{ t('server.warnCrl') }}</p>
            <p v-if="server.warnings.includes('tls-crypt')">{{ t('server.warnTls') }}</p>
          </div>
        </div>
      </section>

      <section v-if="tab === 'clients' && staff" class="panel">
        <h2 class="font-display text-2xl mb-5">{{ t('clients.create') }}</h2>

        <form class="form-stack" @submit.prevent="onCreate">
          <FormField :label="t('clients.email')">
            <input v-model="newEmail" class="input-field" type="email" required />
          </FormField>
          <FormField :label="t('clients.name')">
            <input v-model="newName" class="input-field" :placeholder="t('clients.placeholder')" />
          </FormField>
          <FormField :label="t('clients.password')">
            <input v-model="newPassword" class="input-field" type="password" minlength="8" required />
          </FormField>
          <div class="form-actions">
            <button class="btn btn-primary btn-sm gap-1.5" type="submit">
              <PlusIcon class="size-4" />
              {{ t('clients.add') }}
            </button>
          </div>
        </form>
      </section>

      <section v-if="tab === 'clients' && staff" class="panel">
        <h2 class="font-display text-2xl mb-5">{{ t('clients.list') }}</h2>

        <div class="flex flex-wrap items-center gap-3 mb-4">
          <div class="relative w-full max-w-xs">
            <MagnifyingGlassIcon class="size-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-base-content/40 pointer-events-none" />
            <input v-model="clientQuery" class="input-field input-field-icon" type="search" :placeholder="t('clients.search')" />
          </div>
          <label class="flex items-center gap-2 text-sm cursor-pointer select-none">
            <input v-model="showRevoked" type="checkbox" class="checkbox checkbox-sm" />
            {{ t('clients.showRevoked') }}
          </label>
        </div>

        <p v-if="!filteredClients.length" class="text-base-content/50 text-sm">
          {{
            !clients.length
              ? t('clients.empty')
              : clientQuery
                ? t('clients.emptyFilter')
                : t('clients.emptyActive')
          }}
        </p>
        <div v-else class="overflow-x-auto">
          <table class="table table-sm">
            <thead>
              <tr>
                <th>{{ t('clients.name') }}</th>
                <th>{{ t('clients.email') }}</th>
                <th>{{ t('clients.expires') }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in filteredClients" :key="c.name">
                <td class="font-mono">
                  {{ c.name }}
                  <span v-if="c.revoked || c.disabled" class="badge badge-error badge-xs ml-2">{{ t('clients.revoked') }}</span>
                </td>
                <td class="font-mono text-xs">{{ c.email }}</td>
                <td class="font-mono text-xs">{{ c.not_after?.slice?.(0, 10) }}</td>
                <td class="text-right whitespace-nowrap">
                  <button
                    class="btn btn-ghost btn-xs gap-1"
                    :disabled="c.revoked || c.disabled || !c.has_key"
                    @click="downloadOvpn(c.name)"
                  >
                    <ArrowDownTrayIcon class="size-3.5" />
                    {{ t('clients.download') }}
                  </button>
                  <button class="btn btn-ghost btn-xs gap-1" type="button" @click="onReissue(c.name)">
                    <ArrowPathIcon class="size-3.5" />
                    {{ t('clients.reissue') }}
                  </button>
                  <button
                    class="btn btn-ghost btn-xs gap-1 text-error"
                    :disabled="c.revoked || c.disabled"
                    @click="onRevoke(c.name)"
                  >
                    <NoSymbolIcon class="size-3.5" />
                    {{ t('clients.revoke') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="staff && (tab === 'server' || tab === 'map')" class="panel">
        <div class="flex items-center gap-3 mb-4">
          <h2 class="font-display text-2xl">{{ t('map.title') }}</h2>
          <button class="btn btn-ghost btn-sm gap-1.5 ml-auto" @click="loadConnections(true)">
            <ArrowPathIcon class="size-4" />
            {{ t('map.refresh') }}
          </button>
        </div>
        <p v-if="mapHintText" class="text-warning text-sm mb-4">{{ mapHintText }}</p>
        <p v-else-if="!connections.length" class="text-base-content/50 text-sm mb-4">{{ t('map.empty') }}</p>
        <p v-if="!canKill" class="text-base-content/45 text-xs mb-4">{{ t('map.hintManage') }}</p>
        <div v-if="connections.length" class="overflow-x-auto">
          <table class="table table-sm">
            <thead>
              <tr>
                <th>{{ t('map.user') }}</th>
                <th>{{ t('map.vpn') }}</th>
                <th>{{ t('map.remote') }}</th>
                <th>{{ t('map.location') }}</th>
                <th>{{ t('map.bytesIn') }}</th>
                <th>{{ t('map.bytesOut') }}</th>
                <th>{{ t('map.since') }}</th>
                <th>{{ t('map.lastPing') }}</th>
                <th>{{ t('map.online') }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in connections" :key="c.name + c.real_address">
                <td class="font-mono">{{ c.name }}</td>
                <td class="font-mono text-xs">{{ c.virtual_ip || '—' }}</td>
                <td class="font-mono text-xs">{{ c.real_ip }}</td>
                <td class="text-xs whitespace-nowrap">{{ locationText(c) }}</td>
                <td class="font-mono text-xs whitespace-nowrap">{{ fmtTraffic(c.bytes_received) }}</td>
                <td class="font-mono text-xs whitespace-nowrap">{{ fmtTraffic(c.bytes_sent) }}</td>
                <td class="font-mono text-xs whitespace-nowrap">{{ c.since || '—' }}</td>
                <td class="font-mono text-xs whitespace-nowrap">{{ c.last_ref || '—' }}</td>
                <td class="font-mono text-xs whitespace-nowrap">{{ timeOnline(c) }}</td>
                <td class="text-right whitespace-nowrap">
                  <button
                    class="btn btn-ghost btn-xs gap-1 text-error"
                    type="button"
                    :disabled="!canKill"
                    :title="canKill ? t('map.disconnect') : t('map.hintManage')"
                    @click="onDisconnect(c)"
                  >
                    <XMarkIcon class="size-3.5" />
                    {{ t('map.disconnect') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="tab === 'map' && staff" class="panel">
        <h2 class="font-display text-2xl mb-4">{{ t('map.canvas') }}</h2>
        <MapView :items="connections" />
      </section>

      <SettingsView v-if="tab === 'settings' && staff" @update:state="onSettingsSaved" />
      <UsersView v-if="tab === 'users' && staff" :me="props.me" />
      <HomeView v-if="tab === 'home' && !staff" :me="props.me" />
      <ProfileView v-if="tab === 'profile'" :me="props.me" @update:me="onMe" />

      <section v-if="tab === 'log' && staff" class="panel">
        <div class="flex items-center gap-3 mb-4">
          <h2 class="font-display text-2xl">{{ t('log.title') }}</h2>
          <button class="btn btn-ghost btn-sm gap-1.5 ml-auto" @click="loadLog">
            <ArrowPathIcon class="size-4" />
            {{ t('log.refresh') }}
          </button>
        </div>
        <pre v-if="logText" class="font-mono text-xs whitespace-pre-wrap bg-base-300/60 p-4 rounded-box max-h-[28rem] overflow-auto">{{ logText }}</pre>
        <p v-else-if="logHint === 'unset'" class="text-base-content/50 text-sm">{{ t('log.empty') }}</p>
        <p v-else-if="logHint === 'missing'" class="text-base-content/50 text-sm">{{ t('log.missing') }}</p>
        <p v-else-if="logHint" class="text-warning text-sm">{{ logHint }}</p>
        <p v-else class="text-base-content/50 text-sm">{{ t('log.empty') }}</p>
        <p v-if="logSource === 'journal'" class="text-xs text-base-content/40 mt-2">{{ t('log.journal') }}</p>
      </section>
  </div>
</template>
