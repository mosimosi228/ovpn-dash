<script setup lang="ts">
defineProps<{ label: string }>()
</script>

<template>
  <div class="form-row">
    <label class="form-label">{{ label }}</label>
    <div class="form-control-wrap">
      <slot />
    </div>
  </div>
</template>
