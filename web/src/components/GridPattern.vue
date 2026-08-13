<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    position?: 'absolute' | 'fixed'
    intensity?: 'subtle' | 'normal' | 'strong'
    size?: number
    blobs?: boolean
  }>(),
  { position: 'fixed', intensity: 'normal', size: 32, blobs: true },
)

const intensityClass = computed(() => {
  switch (props.intensity) {
    case 'strong':
      return 'opacity-[0.12]'
    case 'normal':
      return 'opacity-[0.08]'
    default:
      return 'opacity-[0.05]'
  }
})

const containerClass = computed(() =>
  props.position === 'fixed'
    ? 'pointer-events-none fixed inset-0 overflow-hidden'
    : 'pointer-events-none absolute inset-0 -z-10 overflow-hidden',
)

const gridStyle = computed(() => ({
  backgroundSize: `${props.size}px ${props.size}px`,
}))
</script>

<template>
  <div :class="containerClass" :style="{ zIndex: 0 }">
    <div class="absolute inset-0 ovpn-grid" :class="intensityClass" :style="gridStyle" />
    <div
      v-if="blobs"
      class="absolute -top-40 -left-40 w-[40rem] h-[40rem] rounded-full bg-primary/20 blur-3xl animate-[pulse_8s_ease-in-out_infinite]"
    />
    <div
      v-if="blobs"
      class="absolute -bottom-40 -right-40 w-[40rem] h-[40rem] rounded-full bg-accent/12 blur-3xl animate-[pulse_10s_ease-in-out_infinite]"
    />
  </div>
</template>
