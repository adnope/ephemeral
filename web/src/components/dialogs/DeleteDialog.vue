<script setup lang="ts">
import { AlertTriangle, Trash2 } from "@lucide/vue";
import { computed, ref } from "vue";
import { api } from "../../api";
import { publishRealtime } from "../../state/realtime";
import { ui } from "../../state/ui";
import BaseModal from "../BaseModal.vue";

const deleting = ref(false);
const error = ref("");
const title = computed(() =>
  ui.deleteItems.length > 1
    ? `Delete ${ui.deleteItems.length} items?`
    : "Delete this item?",
);

function close() {
  if (!deleting.value) ui.deleteItems = [];
}

async function confirm() {
  deleting.value = true;
  error.value = "";
  const items = [...ui.deleteItems];
  try {
    await Promise.all(items.map((item) => api.deleteItem(item.id)));
    for (const item of items)
      publishRealtime({ type: "item:deleted", id: item.id });
    ui.deleteItems = [];
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Delete failed";
  } finally {
    deleting.value = false;
  }
}
</script>

<template>
  <BaseModal
    :open="ui.deleteItems.length > 0"
    :title="title"
    :close-on-backdrop="!deleting"
    @close="close"
  >
    <div class="confirm-copy">
      <span class="danger-icon"><AlertTriangle :size="24" /></span>
      <p>
        This permanently removes the selected content and any generated media.
        This action cannot be undone.
      </p>
    </div>
    <p v-if="error" class="form-error">{{ error }}</p>
    <template #footer>
      <button
        class="button button-outline"
        type="button"
        :disabled="deleting"
        @click="close"
      >
        Cancel
      </button>
      <button
        class="button button-danger"
        type="button"
        :disabled="deleting"
        @click="confirm"
      >
        <Trash2 :size="16" />{{ deleting ? "Deleting..." : "Delete" }}
      </button>
    </template>
  </BaseModal>
</template>
