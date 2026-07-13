<script setup lang="ts">
import { AlertCircle, LoaderCircle } from "@lucide/vue";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import AppShell from "../components/AppShell.vue";
import ComposerBox from "../components/chat/ComposerBox.vue";
import ItemCard from "../components/items/ItemCard.vue";
import SelectionToolbar from "../components/SelectionToolbar.vue";
import { useItemFeed } from "../composables/useItemFeed";
import { useMarqueeSelection } from "../composables/useMarqueeSelection";
import { api } from "../api";
import { useUploads } from "../state/uploads";
import type { Item } from "../types";

const feed = useItemFeed((cursor) => api.items(cursor));
const selectionActive = ref(false);
const selectedIDs = ref(new Set<number>());
const dragActive = ref(false);
const toast = ref("");
const loadTrigger = ref<HTMLElement | null>(null);
const stream = ref<HTMLElement | null>(null);
const uploads = useUploads();
const selectedItems = computed(() =>
  feed.items.value.filter((item) => selectedIDs.value.has(item.id)),
);
const chatItems = computed(() => [...feed.items.value].reverse());
let loadObserver: IntersectionObserver | null = null;
let loadTriggerVisible = false;
let positionedAtLatest = false;
let loadingOlder = false;
const marquee = useMarqueeSelection(stream, selectionActive, selectedIDs);

onMounted(() => {
  loadObserver = new IntersectionObserver((entries) => {
    loadTriggerVisible = entries.some((entry) => entry.isIntersecting);
    if (loadTriggerVisible) void loadOlder();
  });
  if (loadTrigger.value) loadObserver.observe(loadTrigger.value);
});

onBeforeUnmount(() => loadObserver?.disconnect());

watch(
  () => feed.items.value.map((item) => item.id).join(","),
  async () => {
    const shouldShowLatest =
      !positionedAtLatest || (!loadingOlder && isNearBottom());
    await nextTick();
    if (shouldShowLatest) scrollToLatest();
    positionedAtLatest = true;
  },
);

watch(feed.nextCursor, async () => {
  await nextTick();
  if (loadTriggerVisible) void loadOlder();
});

function isNearBottom() {
  if (!stream.value) return true;
  return (
    stream.value.scrollHeight -
      stream.value.clientHeight -
      stream.value.scrollTop <
    48
  );
}

function scrollToLatest() {
  if (stream.value) stream.value.scrollTop = stream.value.scrollHeight;
}

async function loadOlder() {
  if (
    loadingOlder ||
    feed.loading.value ||
    feed.loadingMore.value ||
    !feed.nextCursor.value ||
    !stream.value
  )
    return;
  loadingOlder = true;
  const previousHeight = stream.value.scrollHeight;
  const previousTop = stream.value.scrollTop;
  await feed.loadMore();
  await nextTick();
  if (stream.value) {
    stream.value.scrollTop =
      previousTop + (stream.value.scrollHeight - previousHeight);
  }
  loadingOlder = false;
}

function toggleSelectionMode() {
  selectionActive.value = !selectionActive.value;
  if (!selectionActive.value) selectedIDs.value = new Set();
}

function toggleItem(item: Item) {
  const next = new Set(selectedIDs.value);
  if (next.has(item.id)) next.delete(item.id);
  else next.add(item.id);
  selectedIDs.value = next;
}

function showError(message: string) {
  toast.value = message;
  setTimeout(() => {
    if (toast.value === message) toast.value = "";
  }, 4500);
}

function onDrop(event: DragEvent) {
  dragActive.value = false;
  const files = [...(event.dataTransfer?.files ?? [])];
  if (files.length) uploads.enqueue(files);
}
</script>

<template>
  <AppShell
    :selection-active="selectionActive"
    @toggle-selection="toggleSelectionMode"
  >
    <section
      class="chat-page"
      :class="{ 'drag-active': dragActive }"
      @dragenter.prevent="dragActive = true"
      @dragover.prevent
      @dragleave.self="dragActive = false"
      @drop.prevent="onDrop"
    >
      <div v-if="dragActive" class="drop-overlay">
        <span>Drop files to upload</span>
      </div>
      <div v-if="toast" class="toast toast-error" role="alert">
        <AlertCircle :size="18" />{{ toast }}
      </div>
      <div
        id="chat-stream"
        ref="stream"
        class="chat-stream"
        @pointerdown="marquee.start"
      >
        <div v-if="feed.loading.value" class="state-panel">
          <LoaderCircle class="spin" />Loading messages...
        </div>
        <div v-else-if="feed.error.value" class="state-panel error">
          <AlertCircle />{{ feed.error.value
          }}<button class="text-button" @click="feed.reset">Retry</button>
        </div>
        <div v-else-if="!feed.items.value.length" class="empty-state">
          <span class="empty-icon">$</span>
          <h2>Ready when you are.</h2>
          <p>Send a message or attach files to start the conversation.</p>
        </div>
        <div ref="loadTrigger" class="load-trigger">
          <LoaderCircle v-if="feed.loadingMore.value" class="spin" />
        </div>
        <ItemCard
          v-for="(item, index) in chatItems"
          :key="item.id"
          :item="item"
          :selected="selectedIDs.has(item.id)"
          :selection-active="selectionActive"
          :media-items="chatItems"
          :menu-above="index === chatItems.length - 1"
          @toggle-selection="toggleItem"
        />
      </div>
      <ComposerBox @created="feed.upsert" @error="showError" />
      <SelectionToolbar
        :items="selectedItems"
        @clear="selectedIDs = new Set()"
        @close="toggleSelectionMode"
      />
    </section>
  </AppShell>
</template>
