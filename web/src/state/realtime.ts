import { onBeforeUnmount, onMounted } from "vue";
import type { Item } from "../types";

export type RealtimeEventType =
  | "item:new"
  | "item:updated"
  | "item:deleted"
  | "reconcile"
  | "item:result";

export interface RealtimeEvent {
  type: RealtimeEventType;
  id?: number;
  item?: Item;
}

type Subscriber = (event: RealtimeEvent) => void;

const subscribers = new Set<Subscriber>();
let source: EventSource | null = null;

function notify(event: RealtimeEvent) {
  for (const subscriber of subscribers) subscriber(event);
}

function connect() {
  if (source || typeof EventSource === "undefined") return;
  source = new EventSource("/api/events");

  for (const type of ["item:new", "item:updated", "item:deleted"] as const) {
    source.addEventListener(type, (raw) => {
      const id = Number.parseInt((raw as MessageEvent<string>).data, 10);
      if (Number.isSafeInteger(id) && id > 0) notify({ type, id });
    });
  }
  source.addEventListener("stream:reset", () => notify({ type: "reconcile" }));
  source.addEventListener("open", () => notify({ type: "reconcile" }));
}

export function publishRealtime(event: RealtimeEvent) {
  notify(event);
}

export function subscribeRealtime(subscriber: Subscriber) {
  subscribers.add(subscriber);
  connect();
  queueMicrotask(() => {
    if (subscribers.has(subscriber)) subscriber({ type: "reconcile" });
  });
  return () => subscribers.delete(subscriber);
}

export function useRealtime(subscriber: Subscriber) {
  let unsubscribe: (() => void) | undefined;
  onMounted(() => {
    unsubscribe = subscribeRealtime(subscriber);
  });
  onBeforeUnmount(() => unsubscribe?.());
}

if (typeof document !== "undefined") {
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") notify({ type: "reconcile" });
  });
}
