<script setup lang="ts">
import { Check, Copy, Link, Trash2 } from "@lucide/vue";
import { ref, watch } from "vue";
import { api } from "../../api";
import { ui } from "../../state/ui";
import type { PublicLinkStatus } from "../../types";
import BaseModal from "../BaseModal.vue";

const status = ref<PublicLinkStatus | null>(null);
const expiry = ref("86400");
const loading = ref(false);
const copied = ref(false);
const error = ref("");

watch(
  () => ui.shareItem,
  async (item) => {
    status.value = null;
    error.value = "";
    if (!item) return;
    loading.value = true;
    try {
      status.value = await api.publicLinkStatus(item.id);
      item.publicLinkActive = status.value.status === "active";
    } catch (caught) {
      error.value =
        caught instanceof Error
          ? caught.message
          : "Could not load sharing state";
    } finally {
      loading.value = false;
    }
  },
  { immediate: true },
);

async function create() {
  if (!ui.shareItem) return;
  loading.value = true;
  error.value = "";
  try {
    const link = await api.createPublicLink(
      ui.shareItem.id,
      expiry.value ? Number(expiry.value) : null,
    );
    status.value = {
      status: "active",
      url: link.url,
      token: link.token,
      expires_at: link.expires_at,
    };
    ui.shareItem.publicLinkActive = true;
  } catch (caught) {
    error.value =
      caught instanceof Error ? caught.message : "Could not create link";
  } finally {
    loading.value = false;
  }
}

async function revoke() {
  if (!ui.shareItem) return;
  loading.value = true;
  try {
    await api.revokePublicLink(ui.shareItem.id);
    status.value = { status: "none", expires_at: null };
    ui.shareItem.publicLinkActive = false;
  } finally {
    loading.value = false;
  }
}

async function copy() {
  if (!status.value?.url) return;
  await navigator.clipboard.writeText(absoluteURL(status.value.url));
  copied.value = true;
  setTimeout(() => (copied.value = false), 1200);
}

function absoluteURL(value: string) {
  return new globalThis.URL(value, globalThis.location.origin).href;
}
</script>

<template>
  <BaseModal
    :open="!!ui.shareItem"
    title="Share file"
    :description="ui.shareItem?.filename"
    @close="ui.shareItem = null"
  >
    <div v-if="loading && !status" class="state-panel">Loading...</div>
    <p v-if="error" class="form-error">{{ error }}</p>
    <template
      v-if="status?.status === 'active' || status?.status === 'expired'"
    >
      <label class="field-label"
        >Public link
        <div class="copy-field">
          <input readonly :value="absoluteURL(status.url ?? '')" /><button
            class="icon-button"
            type="button"
            aria-label="Copy link"
            @click="copy"
          >
            <Check v-if="copied" :size="18" /><Copy v-else :size="18" />
          </button></div
      ></label>
      <p class="muted">
        {{
          status.expires_at
            ? `${status.status === "expired" ? "Expired" : "Expires"} ${new Date(status.expires_at).toLocaleString()}`
            : "Does not expire"
        }}
      </p>
    </template>
    <template v-else-if="status">
      <label class="field-label"
        >Expires<select v-model="expiry">
          <option value="86400">In 1 day</option>
          <option value="604800">In 7 days</option>
          <option value="2592000">In 30 days</option>
          <option value="">Never</option>
        </select></label
      >
    </template>
    <template #footer>
      <button
        class="button button-outline"
        type="button"
        @click="ui.shareItem = null"
      >
        Close
      </button>
      <button
        v-if="status?.status === 'active'"
        class="button button-danger"
        type="button"
        :disabled="loading"
        @click="revoke"
      >
        <Trash2 :size="16" />Revoke
      </button>
      <button
        v-else
        class="button button-primary"
        type="button"
        :disabled="loading || !status"
        @click="create"
      >
        <Link :size="16" />Create link
      </button>
    </template>
  </BaseModal>
</template>
