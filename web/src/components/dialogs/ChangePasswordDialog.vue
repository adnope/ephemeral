<script setup lang="ts">
import { ref } from "vue";
import { api } from "../../api";
import BaseModal from "../BaseModal.vue";

const emit = defineEmits<{ close: [] }>();
const oldPassword = ref("");
const newPassword = ref("");
const confirmation = ref("");
const submitting = ref(false);
const changed = ref(false);
const error = ref("");

function close() {
  if (!submitting.value) emit("close");
}

async function submit() {
  if (submitting.value || changed.value) return;
  error.value = "";
  if (newPassword.value !== confirmation.value) {
    error.value = "New passwords do not match";
    return;
  }
  submitting.value = true;
  try {
    await api.changePassword(oldPassword.value, newPassword.value);
    oldPassword.value = newPassword.value = confirmation.value = "";
    changed.value = true;
  } catch (caught) {
    error.value =
      caught instanceof Error ? caught.message : "Could not change password";
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <BaseModal
    open
    title="Change Password"
    :close-on-backdrop="!submitting"
    @close="close"
  >
    <p v-if="changed" class="muted" role="status">
      Your password has been changed.
    </p>
    <form
      v-else
      id="change-password-form"
      class="auth-form"
      :aria-busy="submitting"
      @submit.prevent="submit"
    >
      <label class="field-label">
        Old password
        <input
          v-model="oldPassword"
          type="password"
          autocomplete="current-password"
          required
          :disabled="submitting"
        />
      </label>
      <label class="field-label">
        New password
        <input
          v-model="newPassword"
          type="password"
          autocomplete="new-password"
          required
          :disabled="submitting"
        />
      </label>
      <label class="field-label">
        Confirm new password
        <input
          v-model="confirmation"
          type="password"
          autocomplete="new-password"
          required
          :disabled="submitting"
        />
      </label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </form>
    <template #footer>
      <button
        class="button button-outline"
        type="button"
        :disabled="submitting"
        @click="close"
      >
        {{ changed ? "Done" : "Cancel" }}
      </button>
      <button
        v-if="!changed"
        class="button button-primary"
        type="submit"
        form="change-password-form"
        :disabled="submitting"
      >
        {{ submitting ? "Changing..." : "Change Password" }}
      </button>
    </template>
  </BaseModal>
</template>
