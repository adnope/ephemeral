<script setup lang="ts">
import { Download, Trash2, X } from "@lucide/vue";
import type { Item } from "../types";
import { requestDelete } from "../state/ui";

const props = defineProps<{ items: Item[] }>();
const emit = defineEmits<{ clear: []; close: [] }>();

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
  <Transition name="selection-toolbar">
    <aside
      v-if="items.length"
      class="selection-toolbar"
      aria-label="Selection actions"
    >
      <strong>{{ items.length }} selected</strong>
      <button
        class="button button-outline"
        type="button"
        @click="emit('clear')"
      >
        <X :size="16" />Clear
      </button>
      <button class="button button-outline" type="button" @click="download">
        <Download :size="16" />Download
      </button>
      <button class="button button-danger" type="button" @click="remove">
        <Trash2 :size="16" />Delete
      </button>
    </aside>
  </Transition>
</template>
