import axios, { type AxiosInstance } from 'axios'
import { errorFromAxios, flash } from '@/lib/flash'

const SETUP_TOKEN_KEY = 'ovpn_setup_token'
const ACCESS_KEY = 'ovpn_access'
const REFRESH_KEY = 'ovpn_refresh'

function readSetupToken(): string {
  let tok = ''
  try {
    tok = (new URLSearchParams(window.location.search).get('setup_token') || '').trim()
  } catch {
    tok = ''
  }
  try {
    if (tok) sessionStorage.setItem(SETUP_TOKEN_KEY, tok)
    else tok = (sessionStorage.getItem(SETUP_TOKEN_KEY) || '').trim()
  } catch {
    /* ignore */
  }
  return tok
}

export const setupToken = readSetupToken()

export function getAccessToken(): string {
  try {
    return localStorage.getItem(ACCESS_KEY) || ''
  } catch {
    return ''
  }
}

export function setTokens(access: string, refresh: string) {
  localStorage.setItem(ACCESS_KEY, access)
  localStorage.setItem(REFRESH_KEY, refresh)
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

function shouldFlash(err: { config?: { url?: string } }): boolean {
  const url = err.config?.url || ''
  if (url.includes('/dashboard/api/state') || url === '/state' || url.endsWith('/api/state')) return false
  return true
}

function wireFlash(client: AxiosInstance) {
  client.interceptors.response.use(
    (r) => r,
    async (err) => {
      if (shouldFlash(err)) {
        flash('error', await errorFromAxios(err))
      }
      return Promise.reject(err)
    },
  )
}

export const dash = axios.create({
  baseURL: '/dashboard/api',
  headers: { Accept: 'application/json' },
})

dash.interceptors.request.use((config) => {
  if (setupToken) config.headers.set('X-Setup-Token', setupToken)
  return config
})
wireFlash(dash)

export const api = axios.create({
  baseURL: '/api/v1',
  headers: { Accept: 'application/json' },
})

api.interceptors.request.use((config) => {
  const t = getAccessToken()
  if (t) config.headers.set('Authorization', `Bearer ${t}`)
  return config
})

api.interceptors.response.use(
  (r) => r,
  async (err) => {
    if (err.response?.status === 401) {
      try {
        const refresh = localStorage.getItem(REFRESH_KEY)
        if (refresh && !err.config?._retry) {
          err.config._retry = true
          const { data } = await axios.post('/auth/refresh', { refresh_token: refresh })
          setTokens(data.access_token, data.refresh_token)
          err.config.headers.Authorization = `Bearer ${data.access_token}`
          return api.request(err.config)
        }
      } catch {
        clearTokens()
      }
    }
    if (shouldFlash(err)) {
      flash('error', await errorFromAxios(err))
    }
    return Promise.reject(err)
  },
)

export async function login(email: string, password: string) {
  try {
    const { data } = await axios.post('/auth/login', { email, username: email, password })
    setTokens(data.access_token, data.refresh_token)
  } catch (e) {
    flash('error', await errorFromAxios(e))
    throw e
  }
}

export async function sendPIN(email: string): Promise<{ expires_at: number; ttl_sec: number }> {
  try {
    const { data } = await axios.post('/auth/login/pin', { email })
    return data
  } catch (e) {
    flash('error', await errorFromAxios(e))
    throw e
  }
}

export async function verifyPIN(email: string, pin: string) {
  try {
    const { data } = await axios.post('/auth/login/pin/verify', { email, pin })
    setTokens(data.access_token, data.refresh_token)
  } catch (e) {
    flash('error', await errorFromAxios(e))
    throw e
  }
}

export async function forgotPassword(email: string) {
  try {
    await axios.post('/auth/forgot', { email })
  } catch (e) {
    flash('error', await errorFromAxios(e))
    throw e
  }
}

export async function resetPassword(token: string, password: string) {
  try {
    await axios.post('/auth/reset', { token, password })
  } catch (e) {
    flash('error', await errorFromAxios(e))
    throw e
  }
}

export type SetupState = {
  complete: boolean
  has_admin: boolean
  pki_dir?: string
  server_conf?: string
  unit?: string
  log_file?: string
  public_host?: string
  warnings?: string[]
  smtp_configured?: boolean
  telegram_configured?: boolean
  telegram_bot_username?: string
  smtp_host?: string
  smtp_port?: string
  smtp_user?: string
  smtp_from?: string
  smtp_tls?: boolean
  smtp_pass_set?: boolean
  telegram_token_set?: boolean
}

export type ServerStatus = {
  active: boolean
  unit: string
  unit_state: string
  pki_dir: string
  server_conf: string
  log_file: string
  public_host: string
  port?: number
  proto?: string
  cipher?: string
  network?: string
  sessions?: number
  has_tls_crypt?: boolean
  has_tls_auth?: boolean
  has_crl_verify?: boolean
  warnings?: string[]
}

export type Client = {
  name: string
  email?: string
  user_id?: number
  not_after: string
  serial: string
  revoked: boolean
  disabled?: boolean
  has_key: boolean
}

export type Me = {
  id: number
  email: string
  name: string
  role: 'root' | 'admin' | 'user'
  client_name?: string
  telegram_bound: boolean
  telegram_bot_username?: string
  theme: 'light' | 'dark'
  map_style: 'auto' | 'light' | 'dark'
  disabled: boolean
}

export type DashUser = Me & { created_at?: string }

export async function fetchMe(): Promise<Me> {
  const { data } = await api.get<Me>('/me')
  return data
}

export async function patchMe(body: Record<string, string>): Promise<Me> {
  const { data } = await api.patch<Me>('/me', body)
  return data
}

export async function bindTelegram(): Promise<{ code: string; bot_username?: string; command: string }> {
  const { data } = await api.post('/me/telegram/bind')
  return data
}

export async function unbindTelegram(): Promise<Me> {
  const { data } = await api.delete<Me>('/me/telegram')
  return data
}

export async function downloadMyOvpn() {
  const { data } = await api.get('/me/ovpn', { responseType: 'blob' })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url
  a.download = 'client.ovpn'
  a.click()
  URL.revokeObjectURL(url)
}

export async function fetchUsers(): Promise<DashUser[]> {
  const { data } = await api.get<{ items: DashUser[] }>('/users')
  return data.items || []
}

export async function createUser(body: Record<string, string>) {
  await api.post('/users', body)
}

export async function patchUser(id: number, body: Record<string, unknown>) {
  await api.patch(`/users/${id}`, body)
}

export async function deleteUser(id: number) {
  await api.delete(`/users/${id}`)
}

export async function fetchState(): Promise<SetupState> {
  const { data } = await dash.get<SetupState>('/state')
  return data
}

export async function postSetup(body: Record<string, unknown>): Promise<SetupState> {
  const { data } = await dash.post<SetupState>('/setup', body)
  return data
}

export async function fetchServer(): Promise<ServerStatus> {
  const { data } = await api.get<ServerStatus>('/server')
  return data
}

export async function startServer(): Promise<ServerStatus> {
  const { data } = await api.post<ServerStatus>('/server/start')
  return data
}

export async function stopServer(): Promise<ServerStatus> {
  const { data } = await api.post<ServerStatus>('/server/stop')
  return data
}

export async function fetchLog(): Promise<{ path: string; text: string; source?: string; hint?: string }> {
  const { data } = await api.get('/server/log')
  return data
}

export async function fetchClients(): Promise<Client[]> {
  const { data } = await api.get<{ items: Client[] }>('/clients')
  return data.items || []
}

export async function createClient(body: { name: string; email: string; password: string; client_name?: string }) {
  await api.post('/clients', body)
}

export async function revokeClient(name: string) {
  await api.delete(`/clients/${encodeURIComponent(name)}`)
}

export async function reissueClient(name: string): Promise<{ ok: boolean; name: string; reload_error?: string }> {
  const { data } = await api.post(`/clients/${encodeURIComponent(name)}/reissue`)
  return data
}

export type Connection = {
  name: string
  real_address: string
  real_ip: string
  virtual_ip?: string
  bytes_received: number
  bytes_sent: number
  since?: string
  since_unix?: number
  last_ref?: string
  country?: string
  country_code?: string
  region?: string
  city?: string
  lat?: number
  lon?: number
}

export async function fetchConnections(): Promise<{
  items: Connection[]
  hint?: string
  status_file?: string
  can_kill?: boolean
}> {
  const { data } = await api.get('/connections')
  return data
}

export async function killConnection(body: { name: string; real_address?: string }) {
  await api.post('/connections/kill', body)
}

export function openConnectionsSocket(
  onData: (data: { items: Connection[]; hint?: string; status_file?: string; can_kill?: boolean }) => void,
): () => void {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${proto}//${location.host}/api/v1/connections/ws`
  let ws: WebSocket | null = null
  let closed = false
  let retry = 0
  let timer = 0

  const connect = () => {
    if (closed) return
    ws = new WebSocket(url)
    ws.onopen = () => {
      retry = 0
      ws?.send(JSON.stringify({ token: getAccessToken() }))
    }
    ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(String(ev.data)) as {
          error?: string
          items?: Connection[]
          hint?: string
          status_file?: string
          can_kill?: boolean
        }
        if (data.error) {
          closed = true
          ws?.close()
          return
        }
        onData({ items: data.items || [], hint: data.hint, status_file: data.status_file, can_kill: data.can_kill })
      } catch {
        /* ignore */
      }
    }
    ws.onclose = () => {
      if (closed) return
      const delay = Math.min(8000, 400 * 2 ** retry)
      retry += 1
      timer = window.setTimeout(connect, delay)
    }
  }
  connect()
  return () => {
    closed = true
    window.clearTimeout(timer)
    ws?.close()
    ws = null
  }
}

export async function downloadOvpn(name: string) {
  const { data } = await api.get(`/clients/${encodeURIComponent(name)}/ovpn`, {
    responseType: 'blob',
  })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url
  a.download = `${name}.ovpn`
  a.click()
  URL.revokeObjectURL(url)
}

export async function fetchSettings(): Promise<SetupState> {
  const { data } = await api.get<SetupState>('/settings')
  return data
}

export async function patchSettings(body: Record<string, unknown>): Promise<SetupState> {
  const { data } = await api.patch<SetupState>('/settings', body)
  return data
}
