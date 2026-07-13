import { reactive } from "vue";
import type { Item } from "../types";

export const ui = reactive({
  deleteItems: [] as Item[],
  shareItem: null as Item | null,
  previewItem: null as Item | null,
  mediaItem: null as Item | null,
  mediaItems: [] as Item[],
});

export function requestDelete(items: Item[]) {
  ui.deleteItems = items;
}

export function openMedia(item: Item, items: Item[]) {
  ui.mediaItem = item;
  ui.mediaItems = items.filter(
    (candidate) => candidate.type === "image" || candidate.type === "video",
  );
}
