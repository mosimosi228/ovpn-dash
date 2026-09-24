<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { Connection } from '@/api/client'

const props = defineProps<{ items: Connection[] }>()
const el = ref<HTMLElement | null>(null)
let map: L.Map | null = null
let layer: L.LayerGroup | null = null
let tilesLayer: L.TileLayer | null = null
let userMoved = false
let fitting = false
let lastSig = ''

function pointSig(items: Connection[]) {
  return items
    .filter((c) => c.lat || c.lon)
    .map((c) => `${c.name}:${c.lat}:${c.lon}`)
    .sort()
    .join('|')
}

function plot() {
  if (!layer) return
  layer.clearLayers()
  const pts: L.LatLngExpression[] = []
  for (const c of props.items) {
    if (!c.lat && !c.lon) continue
    const marker = L.circleMarker([c.lat, c.lon], {
      radius: 8,
      color: '#0f766e',
      fillColor: '#14b8a6',
      fillOpacity: 0.9,
      weight: 2,
    })
    const loc = [c.city, c.region, c.country].filter(Boolean).join(', ')
    marker.bindPopup(`<b>${esc(c.name)}</b><br>${esc(c.real_ip)}<br>${esc(loc)}`)
    marker.addTo(layer)
    pts.push([c.lat, c.lon])
  }
  const sig = pointSig(props.items)
  const changed = sig !== lastSig
  lastSig = sig
  if (changed && !userMoved && pts.length && map) {
    fitting = true
    map.fitBounds(L.latLngBounds(pts).pad(0.35), { maxZoom: 6 })
    fitting = false
  }
}

function setTiles() {
  if (!map) return
  tilesLayer?.remove()
  tilesLayer = L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    maxZoom: 19,
  }).addTo(map)
}

onMounted(() => {
  if (!el.value) return
  map = L.map(el.value, { worldCopyJump: true, zoomControl: true }).setView([25, 20], 2)
  map.on('zoomstart', () => {
    if (!fitting) userMoved = true
  })
  map.on('dragstart', () => {
    userMoved = true
  })
  setTiles()
  layer = L.layerGroup().addTo(map)
  plot()
  requestAnimationFrame(() => map?.invalidateSize())
})

function esc(s: string) {
  return s.replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch] || ch)
}

watch(() => props.items, plot, { deep: true })

onBeforeUnmount(() => {
  map?.remove()
  map = null
  layer = null
  tilesLayer = null
  userMoved = false
  lastSig = ''
})
</script>

<template>
  <div ref="el" class="w-full h-[28rem] rounded-box overflow-hidden border border-base-content/10 bg-base-300" />
</template>

<style>
.leaflet-container {
  font-family: inherit;
  background: #aad3df;
}
html[data-theme='dark'] .leaflet-popup-content-wrapper,
html[data-theme='dark'] .leaflet-popup-tip {
  background: #121a24;
  color: #c5d0dc;
}
html[data-theme='light'] .leaflet-popup-content-wrapper,
html[data-theme='light'] .leaflet-popup-tip {
  background: #ffffff;
  color: #1e293b;
}
</style>
