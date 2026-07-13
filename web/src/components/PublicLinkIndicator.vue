<script setup lang="ts">
import { Link } from "@lucide/vue";
</script>

<template>
  <span
    class="public-link-indicator"
    role="img"
    aria-label="Public link active"
    title="Public link active"
  >
    <Link :size="12" />
  </span>
</template>
