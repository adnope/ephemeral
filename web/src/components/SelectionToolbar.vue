<script setup lang="ts">
import { Download, Eraser, ListChecks, Trash2, X } from "@lucide/vue";
import type { Item } from "../types";
import { requestDelete } from "../state/ui";

const props = defineProps<{
  items: Item[];
  allItems: Item[];
  variant?: "floating" | "composer";
}>();
const emit = defineEmits<{ clear: []; close: []; selectAll: [] }>();

function download() {
  if (!props.items.length) return;
  const ids = props.items.map((item) => item.id).join(",");
  const anchor = document.createElement("a");
  anchor.href = `/api/items/download-zip?ids=${ids}`;
  anchor.download = "ephemeral_download.zip";
  anchor.click();
  emit("close");
}

function remove() {
  if (!props.items.length) return;
  requestDelete(props.items);
  emit("close");
}
</script>

<template>
  <aside
    class="selection-toolbar"
    :class="{ 'selection-toolbar-composer': variant === 'composer' }"
    aria-label="Selection actions"
  >
    <strong>{{ items.length }} selected</strong>
    <button
      class="button button-outline selection-action-button"
      type="button"
      :disabled="!allItems.length || items.length === allItems.length"
      aria-label="Select all"
      title="Select all"
      @click="emit('selectAll')"
    >
      <ListChecks :size="16" /><span class="selection-action-label"
        >Select all</span
      >
    </button>
    <button
      class="button button-outline selection-action-button"
      type="button"
      :disabled="!items.length"
      aria-label="Clear selection"
      title="Clear selection"
      @click="emit('clear')"
    >
      <Eraser :size="16" /><span class="selection-action-label">Clear</span>
    </button>
    <button
      class="button button-outline selection-action-button"
      type="button"
      :disabled="!items.length"
      aria-label="Download selected items"
      title="Download selected items"
      @click="download"
    >
      <Download :size="16" /><span class="selection-action-label"
        >Download</span
      >
    </button>
    <button
      class="button button-danger selection-action-button"
      type="button"
      :disabled="!items.length"
      aria-label="Delete selected items"
      title="Delete selected items"
      @click="remove"
    >
      <Trash2 :size="16" /><span class="selection-action-label">Delete</span>
    </button>
    <button
      class="icon-button selection-close-button"
      type="button"
      aria-label="Exit selection mode"
      title="Exit selection mode"
      @click="emit('close')"
    >
      <X :size="18" />
    </button>
  </aside>
</template>
