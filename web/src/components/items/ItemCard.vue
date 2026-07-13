<script setup lang="ts">
import {
  Check,
  Copy,
  Download,
  EllipsisVertical,
  Eye,
  File,
  Play,
  Share2,
  Trash2,
} from "@lucide/vue";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import type { Item } from "../../types";
import { formatBytes, formatDateTime, linkify } from "../../utils/format";
import { copyImageToClipboard } from "../../utils/clipboard";
import { openMedia, requestDelete, ui } from "../../state/ui";
import PublicLinkIndicator from "../PublicLinkIndicator.vue";

const props = withDefaults(
  defineProps<{
    item: Item;
    layout?: "chat" | "history";
    selected?: boolean;
    selectionActive?: boolean;
    mediaItems?: Item[];
    menuAbove?: boolean;
  }>(),
  {
    layout: "chat",
    selected: false,
    selectionActive: false,
    mediaItems: () => [],
    menuAbove: false,
  },
);
const emit = defineEmits<{ toggleSelection: [item: Item] }>();
const menuOpen = ref(false);
const menuRoot = ref<HTMLElement | null>(null);
const copied = ref(false);
const textSegments = computed(() => linkify(props.item.text));
const poster = computed(
  () =>
    props.item.metadata.thumbnailUrl ||
    (props.item.type === "image" ? props.item.contentUrl : ""),
);

function activate() {
  if (props.selectionActive) emit("toggleSelection", props.item);
  else if (props.item.type === "image" || props.item.type === "video")
    openMedia(props.item, props.mediaItems);
  else if (props.item.type === "file") ui.previewItem = props.item;
}

function toggleMenu() {
  menuOpen.value = !menuOpen.value;
}

function closeMenuOutside(event: PointerEvent) {
  if (!menuRoot.value?.contains(event.target as Node)) menuOpen.value = false;
}

function closeMenuOnEscape(event: KeyboardEvent) {
  if (event.key === "Escape") menuOpen.value = false;
}

watch(menuOpen, (open) => {
  if (open) {
    document.addEventListener("pointerdown", closeMenuOutside, true);
    document.addEventListener("keydown", closeMenuOnEscape);
  } else {
    document.removeEventListener("pointerdown", closeMenuOutside, true);
    document.removeEventListener("keydown", closeMenuOnEscape);
  }
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", closeMenuOutside, true);
  document.removeEventListener("keydown", closeMenuOnEscape);
});

async function copyText() {
  await navigator.clipboard.writeText(props.item.text);
  copied.value = true;
  setTimeout(() => (copied.value = false), 1200);
  menuOpen.value = false;
}

async function copyImage() {
  menuOpen.value = false;
  try {
    await copyImageToClipboard(props.item);
  } catch {
    window.location.href = props.item.downloadUrl;
  }
}
</script>

<template>
  <article
    class="item-card"
    :class="[
      `layout-${layout}`,
      `type-${item.type}`,
      { selected, 'selection-active': selectionActive },
    ]"
    :data-item-id="item.id"
    :data-filename="item.filename"
    @click="selectionActive && emit('toggleSelection', item)"
  >
    <button
      v-if="selectionActive"
      class="selection-indicator"
      type="button"
      :aria-label="selected ? 'Deselect item' : 'Select item'"
      @click.stop="emit('toggleSelection', item)"
    >
      <Check v-if="selected" :size="15" />
    </button>

    <PublicLinkIndicator
      v-if="item.publicLinkActive && layout === 'history'"
      class="history-public-link-indicator"
    />

    <div ref="menuRoot" class="item-menu-wrap">
      <button
        class="icon-button item-menu-button"
        type="button"
        aria-label="Item actions"
        @click.stop="toggleMenu"
      >
        <EllipsisVertical :size="18" />
      </button>
      <div
        v-if="menuOpen"
        class="item-menu"
        :class="{ 'opens-above': menuAbove }"
        role="menu"
      >
        <button
          v-if="item.type === 'text'"
          type="button"
          role="menuitem"
          @click.stop="copyText"
        >
          <Copy :size="15" />{{ copied ? "Copied" : "Copy" }}
        </button>
        <button
          v-if="item.type === 'image'"
          type="button"
          role="menuitem"
          @click.stop="copyImage"
        >
          <Copy :size="15" />Copy image
        </button>
        <a
          v-if="item.type !== 'text'"
          :href="item.downloadUrl"
          :download="item.filename"
          role="menuitem"
          @click.stop="menuOpen = false"
          ><Download :size="15" />Download</a
        >
        <button
          v-if="item.type !== 'text'"
          type="button"
          role="menuitem"
          @click.stop="
            ui.shareItem = item;
            menuOpen = false;
          "
        >
          <Share2 :size="15" />Manage link
        </button>
        <button
          class="danger"
          type="button"
          role="menuitem"
          @click.stop="
            requestDelete([item]);
            menuOpen = false;
          "
        >
          <Trash2 :size="15" />Delete
        </button>
      </div>
    </div>

    <div v-if="item.type === 'text'" class="message-text">
      <template v-for="(segment, index) in textSegments" :key="index">
        <a
          v-if="segment.href"
          :href="segment.href"
          target="_blank"
          rel="noopener noreferrer"
          >{{ segment.text }}</a
        >
        <template v-else>{{ segment.text }}</template>
      </template>
    </div>

    <button
      v-else-if="item.type === 'image'"
      class="media-button"
      type="button"
      :aria-label="`Open ${item.filename}`"
      @click.stop="activate"
    >
      <img :src="poster" :alt="item.filename" loading="lazy" />
    </button>

    <button
      v-else-if="item.type === 'video'"
      class="media-button video-card"
      :class="{ 'has-poster': Boolean(poster) }"
      type="button"
      :aria-label="`Open ${item.filename}`"
      @click.stop="activate"
    >
      <img v-if="poster" :src="poster" :alt="item.filename" loading="lazy" />
      <span v-else class="video-placeholder">Video</span>
      <span class="video-play"><Play :size="19" fill="currentColor" /></span>
      <span v-if="item.metadata.processing" class="processing-badge"
        >Processing</span
      >
    </button>

    <div v-else class="file-card">
      <div class="file-icon"><File :size="24" /></div>
      <div class="file-copy">
        <strong :title="item.filename">{{ item.filename }}</strong>
        <span
          >{{ formatBytes(item.filesizeBytes) }} ·
          {{ item.metadata.mime || "File" }}</span
        >
      </div>
      <div class="file-actions">
        <button
          class="icon-button"
          type="button"
          aria-label="Preview"
          @click.stop="ui.previewItem = item"
        >
          <Eye :size="17" />
        </button>
        <a
          class="icon-button"
          :href="item.downloadUrl"
          :download="item.filename"
          aria-label="Download"
          @click.stop
          ><Download :size="17"
        /></a>
      </div>
    </div>

    <footer class="item-footer">
      <span v-if="item.filename && layout === 'chat'" class="item-filename">
        <span>{{ item.filename }}</span>
        <PublicLinkIndicator v-if="item.publicLinkActive" />
      </span>
      <span v-else />
      <time :datetime="new Date(item.createdAtEpochMillis).toISOString()">{{
        formatDateTime(item.createdAtEpochMillis)
      }}</time>
    </footer>
  </article>
</template>
