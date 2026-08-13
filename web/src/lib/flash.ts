import { reactive } from 'vue'

export type FlashKind = 'error' | 'success' | 'warning'

export type FlashItem = {
  id: number
  kind: FlashKind
  text: string
}

export const flashItems = reactive<FlashItem[]>([])

let seq = 0

export function flash(kind: FlashKind, text: string, ms = 7000) {
  const msg = (text || '').trim()
  if (!msg) return
  const id = ++seq
  flashItems.push({ id, kind, text: msg })
  if (ms > 0) {
    window.setTimeout(() => dismissFlash(id), ms)
  }
}

export function dismissFlash(id: number) {
  const i = flashItems.findIndex((x) => x.id === id)
  if (i >= 0) flashItems.splice(i, 1)
}

export async function errorFromAxios(err: unknown): Promise<string> {
  const e = err as {
    message?: string
    response?: { status?: number; statusText?: string; data?: unknown }
  }
  const data = e.response?.data
  if (data instanceof Blob) {
    try {
      const j = JSON.parse(await data.text()) as { error?: string }
      if (j.error) return j.error
    } catch {
      /* ignore */
    }
  }
  if (data && typeof data === 'object' && 'error' in data) {
    const m = (data as { error?: unknown }).error
    if (typeof m === 'string' && m.trim()) return m
  }
  if (typeof data === 'string' && data.trim() && data.length < 400) return data.trim()
  if (e.response?.statusText) return e.response.statusText
  return e.message || 'request failed'
}
