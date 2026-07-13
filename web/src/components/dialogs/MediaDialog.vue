<script setup lang="ts">
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  Download,
  Share2,
  Trash2,
} from "@lucide/vue";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import Hls from "hls.js";
import { requestDelete, ui } from "../../state/ui";
import { copyImageToClipboard } from "../../utils/clipboard";
import { formatBytes, formatFullDateTime } from "../../utils/format";
import BaseModal from "../BaseModal.vue";
import PublicLinkIndicator from "../PublicLinkIndicator.vue";

const video = ref<HTMLVideoElement | null>(null);
const copying = ref(false);
const copied = ref(false);
const actionError = ref("");
let hls: Hls | null = null;
const index = computed(() =>
  ui.mediaItems.findIndex((item) => item.id === ui.mediaItem?.id),
);
const resolution = computed(() => {
  const metadata = ui.mediaItem?.metadata;
  return metadata?.width && metadata.height
    ? `${metadata.width} × ${metadata.height}`
    : "Unavailable";
});

function move(direction: number) {
  if (!ui.mediaItems.length) return;
  const target =
    (index.value + direction + ui.mediaItems.length) % ui.mediaItems.length;
  ui.mediaItem = ui.mediaItems[target] ?? null;
}

function destroyHls() {
  hls?.destroy();
  hls = null;
}

function keydown(event: KeyboardEvent) {
  if (!ui.mediaItem || ui.mediaItems.length < 2) return;
  if (event.key === "ArrowLeft") move(-1);
  else if (event.key === "ArrowRight") move(1);
  else return;
  event.preventDefault();
}

function remove() {
  if (!ui.mediaItem) return;
  const item = ui.mediaItem;
  ui.mediaItem = null;
  requestDelete([item]);
}

function share() {
  if (!ui.mediaItem) return;
  ui.shareItem = ui.mediaItem;
  ui.mediaItem = null;
}

async function copyImage() {
  if (!ui.mediaItem || ui.mediaItem.type !== "image" || copying.value) return;
  copying.value = true;
  actionError.value = "";
  try {
    await copyImageToClipboard(ui.mediaItem);
    copied.value = true;
    setTimeout(() => (copied.value = false), 1400);
  } catch (caught) {
    actionError.value =
      caught instanceof Error ? caught.message : "Could not copy the image";
  } finally {
    copying.value = false;
  }
}

watch(
  () => ui.mediaItem,
  async (item) => {
    destroyHls();
    copied.value = false;
    actionError.value = "";
    if (!item?.metadata.hlsUrl) return;
    await nextTick();
    if (
      !video.value ||
      video.value.canPlayType("application/vnd.apple.mpegurl")
    )
      return;
    if (Hls.isSupported()) {
      hls = new Hls();
      hls.loadSource(item.metadata.hlsUrl);
      hls.attachMedia(video.value);
    }
  },
  { immediate: true },
);

onMounted(() => window.addEventListener("keydown", keydown));
onBeforeUnmount(() => {
  destroyHls();
  window.removeEventListener("keydown", keydown);
});
</script>

<template>
  <BaseModal
    :open="!!ui.mediaItem"
    :title="ui.mediaItem?.filename ?? 'Media'"
    media
    @close="ui.mediaItem = null"
  >
    <template #title>
      <h2 class="modal-title-row">
        <span>{{ ui.mediaItem?.filename ?? "Media" }}</span>
        <PublicLinkIndicator v-if="ui.mediaItem?.publicLinkActive" />
      </h2>
    </template>
    <div v-if="ui.mediaItem" class="media-dialog-body">
      <div class="media-viewer">
        <button
          v-if="ui.mediaItems.length > 1"
          class="media-nav previous"
          type="button"
          aria-label="Previous media"
          @click="move(-1)"
        >
          <ChevronLeft />
        </button>
        <img
          v-if="ui.mediaItem.type === 'image'"
          :src="ui.mediaItem.contentUrl"
          :alt="ui.mediaItem.filename"
        />
        <div
          v-else-if="ui.mediaItem.metadata.processing"
          class="processing-panel"
        >
          <span class="spin-ring" />
          <h3>Processing video</h3>
          <p>The preview will update automatically when it is ready.</p>
        </div>
        <video
          v-else
          ref="video"
          controls
          autoplay
          playsinline
          :poster="ui.mediaItem.metadata.thumbnailUrl || undefined"
        >
          <source
            v-if="ui.mediaItem.metadata.playbackUrl"
            :src="ui.mediaItem.metadata.playbackUrl"
            :type="ui.mediaItem.metadata.playbackMime || 'video/mp4'"
          />
          <source
            :src="ui.mediaItem.contentUrl"
            :type="ui.mediaItem.metadata.mime"
          />
        </video>
        <button
          v-if="ui.mediaItems.length > 1"
          class="media-nav next"
          type="button"
          aria-label="Next media"
          @click="move(1)"
        >
          <ChevronRight />
        </button>
      </div>
      <dl class="media-metadata">
        <div>
          <dt>Resolution</dt>
          <dd>{{ resolution }}</dd>
        </div>
        <div>
          <dt>Size</dt>
          <dd>{{ formatBytes(ui.mediaItem.filesizeBytes) }}</dd>
        </div>
        <div>
          <dt>Uploaded</dt>
          <dd>{{ formatFullDateTime(ui.mediaItem.createdAtEpochMillis) }}</dd>
        </div>
        <div>
          <dt>Type</dt>
          <dd>{{ ui.mediaItem.metadata.mime || ui.mediaItem.type }}</dd>
        </div>
        <div v-if="ui.mediaItem.metadata.duration">
          <dt>Duration</dt>
          <dd>{{ ui.mediaItem.metadata.duration }}</dd>
        </div>
        <div v-if="ui.mediaItems.length > 1">
          <dt>Position</dt>
          <dd>{{ index + 1 }} of {{ ui.mediaItems.length }}</dd>
        </div>
      </dl>
    </div>
    <template #footer>
      <p v-if="actionError" class="media-action-error">{{ actionError }}</p>
      <span class="modal-footer-spacer" />
      <button class="button button-danger" type="button" @click="remove">
        <Trash2 :size="16" />Delete
      </button>
      <button class="button button-outline" type="button" @click="share">
        <Share2 :size="16" />{{
          ui.mediaItem?.publicLinkActive ? "Manage link" : "Share"
        }}
      </button>
      <button
        v-if="ui.mediaItem?.type === 'image'"
        class="button button-outline"
        type="button"
        :disabled="copying"
        @click="copyImage"
      >
        <Check v-if="copied" :size="16" />
        <Copy v-else :size="16" />{{ copied ? "Copied" : "Copy" }}
      </button>
      <a
        v-if="ui.mediaItem"
        class="button button-primary"
        :href="ui.mediaItem.downloadUrl"
        :download="ui.mediaItem.filename"
      >
        <Download :size="16" />Download
      </a>
    </template>
  </BaseModal>
</template>
