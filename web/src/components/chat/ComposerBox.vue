<script setup lang="ts">
import { File, Paperclip, Send, X } from "@lucide/vue";
import { computed, nextTick, onBeforeUnmount, ref } from "vue";
import { api } from "../../api";
import { useUploads } from "../../state/uploads";
import type { Item } from "../../types";

interface Attachment {
  id: number;
  file: File;
  preview: string;
}

const emit = defineEmits<{ created: [item: Item]; error: [message: string] }>();
const uploads = useUploads();
const message = ref("");
const attachments = ref<Attachment[]>([]);
const sending = ref(false);
const input = ref<HTMLTextAreaElement | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
let sequence = 0;
const canSend = computed(
  () => message.value.trim().length > 0 || attachments.value.length > 0,
);

function addFiles(files: File[]) {
  for (const file of files) {
    const relative = file.webkitRelativePath ?? "";
    if (relative.includes("/")) {
      emit("error", "Folders cannot be attached. Select files instead.");
      continue;
    }
    attachments.value.push({
      id: ++sequence,
      file,
      preview:
        file.type.startsWith("image/") || file.type.startsWith("video/")
          ? URL.createObjectURL(file)
          : "",
    });
  }
}

function removeAttachment(attachment: Attachment) {
  if (attachment.preview) URL.revokeObjectURL(attachment.preview);
  attachments.value = attachments.value.filter(
    (candidate) => candidate.id !== attachment.id,
  );
}

async function send() {
  if (!canSend.value || sending.value) return;
  const text = message.value.trim();
  const files = attachments.value.map((attachment) => attachment.file);
  for (const attachment of attachments.value)
    if (attachment.preview) URL.revokeObjectURL(attachment.preview);
  attachments.value = [];
  message.value = "";
  sending.value = true;
  try {
    if (text) emit("created", await api.message(text));
    uploads.enqueue(files);
  } catch (error) {
    message.value = text;
    emit(
      "error",
      error instanceof Error ? error.message : "Message could not be sent",
    );
  } finally {
    sending.value = false;
    await nextTick();
    input.value?.focus();
  }
}

function keydown(event: KeyboardEvent) {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}

onBeforeUnmount(() => {
  for (const attachment of attachments.value)
    if (attachment.preview) URL.revokeObjectURL(attachment.preview);
});
</script>

<template>
  <section class="composer-shell">
    <div v-if="attachments.length" class="attachment-tray">
      <article
        v-for="attachment in attachments"
        :key="attachment.id"
        class="attachment-chip"
      >
        <img
          v-if="attachment.preview && attachment.file.type.startsWith('image/')"
          :src="attachment.preview"
          alt=""
        />
        <video v-else-if="attachment.preview" :src="attachment.preview" muted />
        <span v-else class="attachment-file-icon"><File :size="20" /></span>
        <span class="attachment-name">{{ attachment.file.name }}</span>
        <button
          class="icon-button"
          type="button"
          aria-label="Remove attachment"
          @click="removeAttachment(attachment)"
        >
          <X :size="15" />
        </button>
      </article>
    </div>
    <div class="composer-row">
      <input
        ref="fileInput"
        class="visually-hidden"
        type="file"
        multiple
        @change="
          addFiles(Array.from(($event.target as HTMLInputElement).files ?? []));
          ($event.target as HTMLInputElement).value = '';
        "
      />
      <button
        class="icon-button composer-attach"
        type="button"
        aria-label="Attach files"
        @click="fileInput?.click()"
      >
        <Paperclip :size="20" />
      </button>
      <textarea
        ref="input"
        v-model="message"
        rows="1"
        placeholder="Type a message..."
        aria-label="Message"
        @keydown="keydown"
      />
      <button
        class="button button-primary composer-send"
        type="button"
        :disabled="!canSend || sending"
        aria-label="Send"
        @click="send"
      >
        <Send :size="18" />
      </button>
    </div>
  </section>
</template>
