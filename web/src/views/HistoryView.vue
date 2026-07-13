<script setup lang="ts">
import { AlertCircle, LoaderCircle, Search, X } from "@lucide/vue";
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import AppShell from "../components/AppShell.vue";
import ItemCard from "../components/items/ItemCard.vue";
import SelectionToolbar from "../components/SelectionToolbar.vue";
import { useItemFeed } from "../composables/useItemFeed";
import { useMarqueeSelection } from "../composables/useMarqueeSelection";
import { api } from "../api";
import type { HistoryFilters, Item } from "../types";

const route = useRoute();
const router = useRouter();
const filters = reactive<HistoryFilters>({
  type: "",
  q: "",
  body: false,
  from: "",
  to: "",
  recent: "",
  visibility: "",
});
const selectionActive = ref(false);
const selectedIDs = ref(new Set<number>());
const loadTrigger = ref<HTMLElement | null>(null);
const historyPage = ref<HTMLElement | null>(null);
const feed = useItemFeed((cursor) => api.history(filters, cursor), true);
const selectedItems = computed(() =>
  feed.items.value.filter((item) => selectedIDs.value.has(item.id)),
);
let loadObserver: IntersectionObserver | null = null;
let loadTriggerVisible = false;
const marquee = useMarqueeSelection(historyPage, selectionActive, selectedIDs);

function readRoute() {
  filters.type = typeof route.query.type === "string" ? route.query.type : "";
  filters.q = typeof route.query.q === "string" ? route.query.q : "";
  filters.body = route.query.body === "1";
  filters.from = typeof route.query.from === "string" ? route.query.from : "";
  filters.to = typeof route.query.to === "string" ? route.query.to : "";
  filters.recent =
    typeof route.query.recent === "string" ? route.query.recent : "";
  filters.visibility =
    typeof route.query.visibility === "string" ? route.query.visibility : "";
}

function applyFilters() {
  const query: Record<string, string> = {};
  if (filters.type) query.type = filters.type;
  if (filters.q) query.q = filters.q;
  if (filters.body) query.body = "1";
  if (filters.from) query.from = filters.from;
  if (filters.to) query.to = filters.to;
  if (filters.recent) query.recent = filters.recent;
  if (filters.visibility) query.visibility = filters.visibility;
  void router.replace({ query });
}

function setType(type: string) {
  filters.type = type;
  applyFilters();
}

function clearFilters() {
  Object.assign(filters, {
    type: "",
    q: "",
    body: false,
    from: "",
    to: "",
    recent: "",
    visibility: "",
  });
  applyFilters();
}

function toggleItem(item: Item) {
  const next = new Set(selectedIDs.value);
  if (next.has(item.id)) next.delete(item.id);
  else next.add(item.id);
  selectedIDs.value = next;
}

readRoute();
watch(
  () => route.fullPath,
  () => {
    readRoute();
    selectedIDs.value = new Set();
    void feed.reset();
  },
);
onMounted(() => {
  loadObserver = new IntersectionObserver((entries) => {
    loadTriggerVisible = entries.some((entry) => entry.isIntersecting);
    if (loadTriggerVisible) void feed.loadMore();
  });
  if (loadTrigger.value) loadObserver.observe(loadTrigger.value);
});
onBeforeUnmount(() => loadObserver?.disconnect());
watch(feed.nextCursor, () => {
  if (loadTriggerVisible) void feed.loadMore();
});
</script>

<template>
  <AppShell
    :selection-active="selectionActive"
    @toggle-selection="
      selectionActive = !selectionActive;
      selectedIDs = new Set();
    "
  >
    <section
      ref="historyPage"
      class="history-page"
      @pointerdown="marquee.start"
    >
      <p class="history-count">{{ feed.items.value.length }} loaded</p>
      <form class="filter-panel" @submit.prevent="applyFilters">
        <label class="search-field"
          ><Search :size="18" /><input
            v-model="filters.q"
            type="search"
            placeholder="Search filenames"
        /></label>
        <button class="button button-primary" type="submit">Search</button>
        <button
          v-if="Object.values(filters).some(Boolean)"
          class="button button-outline"
          type="button"
          @click="clearFilters"
        >
          <X :size="16" />Clear
        </button>
        <div class="filter-details">
          <label
            >From<input
              v-model="filters.from"
              type="date"
              @change="applyFilters"
          /></label>
          <label
            >To<input v-model="filters.to" type="date" @change="applyFilters"
          /></label>
          <label
            >Recent<select v-model="filters.recent" @change="applyFilters">
              <option value="">Any time</option>
              <option
                v-for="value in ['1d', '7d', '14d', '30d', '90d', '6mo', '1y']"
                :key="value"
                :value="value"
              >
                {{ value }}
              </option>
            </select></label
          >
          <label
            >Visibility<select
              v-model="filters.visibility"
              @change="applyFilters"
            >
              <option value="">All</option>
              <option value="private">Private</option>
              <option value="public">Public</option>
            </select></label
          >
          <label class="checkbox-field"
            ><input
              v-model="filters.body"
              type="checkbox"
              @change="applyFilters"
            />Search text/code file body</label
          >
        </div>
      </form>
      <nav class="filter-pills" aria-label="Item type filters">
        <button
          v-for="option in [
            { value: '', label: 'All' },
            { value: 'image', label: 'Images' },
            { value: 'video', label: 'Videos' },
            { value: 'file', label: 'Files' },
          ]"
          :key="option.value"
          type="button"
          :class="{ active: filters.type === option.value }"
          @click="setType(option.value)"
        >
          {{ option.label }}
        </button>
      </nav>

      <div v-if="feed.loading.value" class="state-panel">
        <LoaderCircle class="spin" />Loading history...
      </div>
      <div v-else-if="feed.error.value" class="state-panel error">
        <AlertCircle />{{ feed.error.value }}
      </div>
      <div v-else-if="!feed.items.value.length" class="empty-state">
        <Search :size="28" />
        <h2>No items found.</h2>
        <p>Try adjusting the filters or upload something new.</p>
      </div>
      <div v-else id="gallery-grid" class="history-grid">
        <ItemCard
          v-for="item in feed.items.value"
          :key="item.id"
          :item="item"
          layout="history"
          :selected="selectedIDs.has(item.id)"
          :selection-active="selectionActive"
          :media-items="feed.items.value"
          @toggle-selection="toggleItem"
        />
      </div>
      <div ref="loadTrigger" class="load-trigger">
        <LoaderCircle v-if="feed.loadingMore.value" class="spin" />
      </div>
      <SelectionToolbar
        :items="selectedItems"
        @clear="selectedIDs = new Set()"
        @close="
          selectionActive = false;
          selectedIDs = new Set();
        "
      />
    </section>
  </AppShell>
</template>
