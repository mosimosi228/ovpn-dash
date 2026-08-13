<script setup lang="ts">
import { flashItems, dismissFlash } from '@/lib/flash'

function cls(kind: string) {
  if (kind === 'success') return 'alert-success'
  if (kind === 'warning') return 'alert-warning'
  return 'alert-error'
}
</script>

<template>
  <div class="toast toast-top toast-end z-[80] mt-16">
    <div
      v-for="f in flashItems"
      :key="f.id"
      class="alert shadow-lg max-w-sm text-sm cursor-pointer"
      :class="cls(f.kind)"
      role="status"
      @click="dismissFlash(f.id)"
    >
      <span class="whitespace-pre-wrap break-words">{{ f.text }}</span>
    </div>
  </div>
</template>
