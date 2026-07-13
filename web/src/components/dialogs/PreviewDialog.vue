<script setup lang="ts">
import { Check, Copy, Download, Share2, Trash2 } from "@lucide/vue";
import { ref, watch } from "vue";
import { api } from "../../api";
import { highlightCode, highlightLanguageOptions } from "../../highlighter";
import { requestDelete, ui } from "../../state/ui";
import type { FilePreview } from "../../types";
import { formatBytes } from "../../utils/format";
import BaseModal from "../BaseModal.vue";
import PublicLinkIndicator from "../PublicLinkIndicator.vue";

const preview = ref<FilePreview | null>(null);
const highlighted = ref("");
const loading = ref(false);
const error = ref("");
const actionError = ref("");
const selectedLanguage = ref("text");
const copied = ref(false);
let request = 0;
let highlightRequest = 0;

async function renderHighlight(source: string, language: string) {
  const current = ++highlightRequest;
  try {
    const result = await highlightCode(source, language);
    if (current === highlightRequest) highlighted.value = result;
  } catch (caught) {
    if (current === highlightRequest) {
      actionError.value =
        caught instanceof Error ? caught.message : "Highlighting failed";
    }
  }
}

watch(
  () => ui.previewItem,
  async (item) => {
    const current = ++request;
    preview.value = null;
    highlighted.value = "";
    error.value = "";
    actionError.value = "";
    copied.value = false;
    if (!item) return;
    loading.value = true;
    try {
      const result = await api.preview(item.id);
      if (current !== request) return;
      preview.value = result;
      selectedLanguage.value = result.language || "text";
      await renderHighlight(result.content, selectedLanguage.value);
    } catch (caught) {
      error.value =
        caught instanceof Error ? caught.message : "Preview unavailable";
    } finally {
      if (current === request) loading.value = false;
    }
  },
  { immediate: true },
);

function changeLanguage() {
  if (preview.value)
    void renderHighlight(preview.value.content, selectedLanguage.value);
}

async function copy() {
  if (!preview.value) return;
  actionError.value = "";
  try {
    await navigator.clipboard.writeText(preview.value.content);
    copied.value = true;
    setTimeout(() => (copied.value = false), 1400);
  } catch (caught) {
    actionError.value =
      caught instanceof Error ? caught.message : "Could not copy the file";
  }
}

function share() {
  if (!ui.previewItem) return;
  ui.shareItem = ui.previewItem;
  ui.previewItem = null;
}

function remove() {
  if (!ui.previewItem) return;
  const item = ui.previewItem;
  ui.previewItem = null;
  requestDelete([item]);
}
</script>

<template>
  <BaseModal
    :open="!!ui.previewItem"
    :title="ui.previewItem?.filename ?? 'File preview'"
    wide
    @close="ui.previewItem = null"
  >
    <template #title>
      <h2 class="modal-title-row">
        <span>{{ ui.previewItem?.filename ?? "File preview" }}</span>
        <PublicLinkIndicator v-if="ui.previewItem?.publicLinkActive" />
      </h2>
    </template>
    <div v-if="loading" class="state-panel">Loading preview...</div>
    <div v-else-if="error" class="state-panel error">{{ error }}</div>
    <template v-else-if="preview">
      <div class="preview-toolbar">
        <div class="preview-meta">
          <span>{{ preview.mime }}</span
          ><span>{{ formatBytes(preview.filesize) }}</span
          ><span>{{ preview.created_at }}</span>
        </div>
        <label class="preview-language"
          ><span>Language</span
          ><select v-model="selectedLanguage" @change="changeLanguage">
            <option
              v-for="option in highlightLanguageOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select></label
        >
      </div>
      <p v-if="actionError" class="form-error">{{ actionError }}</p>
      <!-- The HTML is generated locally by Shiki from escaped source text. -->
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div class="code-preview" v-html="highlighted" />
    </template>
    <template #footer>
      <button
        class="button button-outline"
        type="button"
        @click="ui.previewItem = null"
      >
        Close
      </button>
      <span class="modal-footer-spacer" />
      <button
        v-if="ui.previewItem"
        class="button button-danger"
        type="button"
        @click="remove"
      >
        <Trash2 :size="16" />Delete
      </button>
      <button
        v-if="ui.previewItem"
        class="button button-outline"
        type="button"
        @click="share"
      >
        <Share2 :size="16" />{{
          ui.previewItem.publicLinkActive ? "Manage link" : "Share"
        }}
      </button>
      <button
        v-if="preview"
        class="button button-outline"
        type="button"
        @click="copy"
      >
        <Check v-if="copied" :size="16" />
        <Copy v-else :size="16" />{{ copied ? "Copied" : "Copy" }}
      </button>
      <a
        v-if="preview"
        class="button button-primary"
        :href="preview.download_url"
        :download="preview.filename"
        ><Download :size="16" />Download</a
      >
    </template>
  </BaseModal>
</template>
