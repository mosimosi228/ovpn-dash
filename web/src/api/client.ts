import axios from 'axios'

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

export const dash = axios.create({
  baseURL: '/dashboard/api',
  headers: { Accept: 'application/json' },
})

dash.interceptors.request.use((config) => {
  if (setupToken) config.headers.set('X-Setup-Token', setupToken)
  return config
})

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
    return Promise.reject(err)
  },
)

export type SetupState = {
  complete: boolean
  has_admin: boolean
  admin_user?: string
  pki_dir?: string
  server_conf?: string
  unit?: string
  log_file?: string
  public_host?: string
  warnings?: string[]
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
  has_tls_crypt?: boolean
  has_tls_auth?: boolean
  has_crl_verify?: boolean
  warnings?: string[]
}

export type Client = {
  name: string
  not_after: string
  serial: string
  revoked: boolean
  has_key: boolean
}

export async function fetchState(): Promise<SetupState> {
  const { data } = await dash.get<SetupState>('/state')
  return data
}

export async function postSetup(body: Record<string, unknown>): Promise<SetupState> {
  const { data } = await dash.post<SetupState>('/setup', body)
  return data
}

export async function login(username: string, password: string) {
  const { data } = await axios.post('/auth/login', { username, password })
  setTokens(data.access_token, data.refresh_token)
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

export async function createClient(name: string) {
  await api.post('/clients', { name })
}

export async function revokeClient(name: string) {
  await api.delete(`/clients/${encodeURIComponent(name)}`)
}

export type Connection = {
  name: string
  real_address: string
  real_ip: string
  virtual_ip?: string
  bytes_received: number
  bytes_sent: number
  since?: string
  country?: string
  city?: string
  lat?: number
  lon?: number
}

export async function fetchConnections(): Promise<{ items: Connection[]; hint?: string; status_file?: string }> {
  const { data } = await api.get('/connections')
  return data
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

export async function patchSettings(body: Record<string, string>): Promise<SetupState> {
  const { data } = await api.patch<SetupState>('/settings', body)
  return data
}
