<script setup lang="ts">
import { ref } from "vue";
import { api } from "../../api";
import { publishRealtime } from "../../state/realtime";
import type { Item } from "../../types";
import BaseModal from "../BaseModal.vue";

const props = defineProps<{ item: Item }>();
const emit = defineEmits<{ close: [] }>();
const text = ref(props.item.text);
const saving = ref(false);
const error = ref("");

function close() {
  if (!saving.value) emit("close");
}

async function save() {
  if (saving.value) return;
  error.value = "";
  const trimmed = text.value.trim();
  if (!trimmed) {
    error.value = "Text must not be empty";
    return;
  }
  saving.value = true;
  try {
    const item = await api.editMessage(props.item.id, trimmed);
    publishRealtime({ type: "item:result", id: item.id, item });
    emit("close");
  } catch (caught) {
    error.value =
      caught instanceof Error ? caught.message : "Could not save changes";
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <BaseModal
    open
    title="Edit text"
    panel-class="edit-message-modal"
    :close-on-backdrop="!saving"
    @close="close"
  >
    <form id="edit-message-form" :aria-busy="saving" @submit.prevent="save">
      <label class="field-label">
        Text
        <textarea
          v-model="text"
          class="edit-message-textarea"
          rows="8"
          required
          :disabled="saving"
        />
      </label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button
        class="button button-outline"
        type="button"
        :disabled="saving"
        @click="close"
      >
        Cancel
      </button>
      <button
        class="button button-primary"
        type="submit"
        form="edit-message-form"
        :disabled="saving || !text.trim()"
      >
        {{ saving ? "Saving..." : "Save" }}
      </button>
    </template>
  </BaseModal>
</template>
