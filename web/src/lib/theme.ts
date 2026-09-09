export type Theme = 'light' | 'dark'
export type MapStyle = 'auto' | 'light' | 'dark'

export function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return
  document.documentElement.setAttribute('data-theme', theme)
}

export function mapTiles(theme: Theme, mapStyle: MapStyle): 'light' | 'dark' {
  if (mapStyle === 'light' || mapStyle === 'dark') return mapStyle
  return theme
}

applyTheme('light')
