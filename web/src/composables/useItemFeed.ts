import { onBeforeUnmount, ref, type Ref } from "vue";
import { api, APIError } from "../api";
import { useRealtime, type RealtimeEvent } from "../state/realtime";
import type { Item, ItemPage } from "../types";

export function useItemFeed(
  loader: (cursor: number) => Promise<ItemPage>,
  refreshOnChange = false,
) {
  const items = ref<Item[]>([]);
  const nextCursor = ref(0);
  const loading = ref(false);
  const loadingMore = ref(false);
  const error = ref("");
  let generation = 0;
  let reconcileTimer = 0;
  const itemVersions = new Map<number, number>();

  async function reset() {
    const current = ++generation;
    loading.value = true;
    error.value = "";
    try {
      const page = await loader(0);
      if (current !== generation) return;
      items.value = page.items.sort((left, right) => right.id - left.id);
      nextCursor.value = page.nextCursor;
    } catch (caught) {
      if (caught instanceof APIError && caught.status === 401) {
        window.location.href = "/login";
        return;
      }
      error.value =
        caught instanceof Error ? caught.message : "Could not load items";
    } finally {
      if (current === generation) loading.value = false;
    }
  }

  async function loadMore() {
    if (!nextCursor.value || loadingMore.value) return;
    loadingMore.value = true;
    try {
      const page = await loader(nextCursor.value);
      const seen = new Set(items.value.map((item) => item.id));
      items.value.push(...page.items.filter((item) => !seen.has(item.id)));
      items.value.sort((left, right) => right.id - left.id);
      nextCursor.value = page.nextCursor;
    } catch (caught) {
      error.value =
        caught instanceof Error ? caught.message : "Could not load more items";
    } finally {
      loadingMore.value = false;
    }
  }

  function upsert(item: Item) {
    const index = items.value.findIndex(
      (candidate) => candidate.id === item.id,
    );
    if (index >= 0) items.value[index] = item;
    else items.value.unshift(item);
    items.value.sort((left, right) => right.id - left.id);
  }

  function remove(id: number) {
    items.value = items.value.filter((item) => item.id !== id);
  }

  async function syncItem(id: number, version: number) {
    try {
      const item = await api.item(id);
      if (itemVersions.get(id) === version) upsert(item);
    } catch (caught) {
      if (caught instanceof APIError && caught.status === 404) remove(id);
      else void reset();
    }
  }

  function handleRealtime(event: RealtimeEvent) {
    if (event.type === "reconcile") {
      window.clearTimeout(reconcileTimer);
      reconcileTimer = window.setTimeout(() => void reset(), 25);
      return;
    }
    if (event.type === "item:result" && event.item) {
      upsert(event.item);
      return;
    }
    if (!event.id) return;
    const version = (itemVersions.get(event.id) ?? 0) + 1;
    itemVersions.set(event.id, version);
    if (event.type === "item:deleted") {
      remove(event.id);
    } else if (refreshOnChange) {
      void reset();
    } else {
      void syncItem(event.id, version);
    }
  }

  useRealtime(handleRealtime);
  onBeforeUnmount(() => window.clearTimeout(reconcileTimer));

  return {
    items: items as Ref<Item[]>,
    nextCursor,
    loading,
    loadingMore,
    error,
    reset,
    loadMore,
    upsert,
    remove,
  };
}
