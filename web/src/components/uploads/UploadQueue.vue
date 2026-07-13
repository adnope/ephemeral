<script setup lang="ts">
import { ChevronDown, RotateCcw, Upload as UploadIcon, X } from "@lucide/vue";
import { computed } from "vue";
import { formatBytes } from "../../utils/format";
import { useUploads } from "../../state/uploads";

const uploads = useUploads();
const summary = computed(() => {
  const total = uploads.state.entries.length;
  const failed = uploads.state.entries.filter(
    (entry) => entry.status === "failed",
  ).length;
  const done = uploads.state.entries.filter(
    (entry) => entry.status === "done",
  ).length;
  const active = uploads.state.entries.filter(
    (entry) => entry.status === "uploading",
  ).length;
  if (active)
    return `${active} uploading · ${formatBytes(uploads.totalBytes.value)}`;
  if (failed) return `${failed} failed · ${done}/${total} done`;
  return `${total} completed · ${formatBytes(uploads.totalBytes.value)}`;
});
</script>

<template>
  <Transition name="upload-queue">
    <section
      v-if="uploads.state.visible && uploads.state.entries.length"
      class="upload-queue"
      :class="{ 'is-collapsed': uploads.state.collapsed }"
      aria-label="Upload queue"
    >
      <button
        v-if="uploads.state.collapsed"
        class="upload-queue-collapsed"
        type="button"
        :aria-label="`Expand upload queue. ${summary}`"
        :title="`Uploads - ${summary}`"
        @click="uploads.state.collapsed = false"
      >
        <UploadIcon :size="21" />
      </button>
      <template v-else>
        <header class="upload-queue-header">
          <button
            class="upload-queue-toggle"
            type="button"
            aria-expanded="true"
            @click="uploads.state.collapsed = true"
          >
            <span
              ><strong>Uploads</strong><small>{{ summary }}</small></span
            >
            <ChevronDown :size="17" />
          </button>
          <button
            class="icon-button"
            type="button"
            aria-label="Close upload queue"
            @click="uploads.close"
          >
            <X :size="18" />
          </button>
        </header>
        <div class="progress-track overall-progress">
          <span :style="{ width: `${uploads.overallProgress.value}%` }" />
        </div>
        <div class="upload-list">
          <article
            v-for="entry in uploads.state.entries"
            :key="entry.id"
            class="upload-entry"
          >
            <div class="upload-entry-row">
              <div class="upload-entry-copy">
                <strong :title="entry.name">{{ entry.name }}</strong>
                <small
                  >{{ formatBytes(entry.size) }} · {{ entry.status }}</small
                >
              </div>
              <button
                v-if="entry.status === 'queued' || entry.status === 'uploading'"
                class="text-button"
                type="button"
                @click="uploads.cancel(entry)"
              >
                Cancel
              </button>
              <button
                v-if="entry.status === 'failed' || entry.status === 'canceled'"
                class="icon-button"
                type="button"
                aria-label="Retry upload"
                @click="uploads.retry(entry)"
              >
                <RotateCcw :size="16" />
              </button>
            </div>
            <div class="progress-track" :class="entry.status">
              <span :style="{ width: `${entry.progress}%` }" />
            </div>
            <p v-if="entry.error" class="upload-error">{{ entry.error }}</p>
          </article>
          <button
            v-if="uploads.hasFinished.value"
            class="text-button clear-uploads"
            type="button"
            @click="uploads.clearFinished"
          >
            Clear completed
          </button>
        </div>
      </template>
    </section>
  </Transition>
</template>
