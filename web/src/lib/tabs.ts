export type DashTab = 'home' | 'server' | 'clients' | 'map' | 'log' | 'settings' | 'users' | 'profile'

export const STAFF_TABS: DashTab[] = ['server', 'clients', 'map', 'log', 'settings', 'users', 'profile']
export const USER_TABS: DashTab[] = ['home', 'profile']

function dashBase(): string {
  const raw = import.meta.env.BASE_URL || '/dashboard/'
  return raw.replace(/\/+$/, '') || '/dashboard'
}

export function urlForTab(tab: DashTab): string {
  return `${dashBase()}/${tab}`
}

export function defaultTab(staff: boolean): DashTab {
  return staff ? 'server' : 'home'
}

export function tabFromPath(staff: boolean, pathname = window.location.pathname): DashTab {
  const allowed = new Set<string>(staff ? STAFF_TABS : USER_TABS)
  const base = dashBase()
  const rest = pathname.replace(/\/+$/, '')
  if (!rest || rest === base) return defaultTab(staff)
  if (!rest.startsWith(`${base}/`)) return defaultTab(staff)
  const seg = rest.slice(base.length + 1).split('/')[0] || ''
  if (allowed.has(seg)) return seg as DashTab
  return defaultTab(staff)
}

export function writeTabUrl(tab: DashTab, replace = false) {
  const next = urlForTab(tab) + window.location.search + window.location.hash
  const curPath = window.location.pathname.replace(/\/+$/, '')
  const wantPath = urlForTab(tab).replace(/\/+$/, '')
  if (curPath === wantPath) {
    if (replace) history.replaceState({ tab }, '', next)
    return
  }
  if (replace) history.replaceState({ tab }, '', next)
  else history.pushState({ tab }, '', next)
}
