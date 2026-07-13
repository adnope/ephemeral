import { computed, reactive } from "vue";
import { APIError } from "../api";
import { loadRuntimeConfig } from "./config";
import { publishRealtime } from "./realtime";
import type { Item } from "../types";

export type UploadStatus =
  | "queued"
  | "uploading"
  | "done"
  | "failed"
  | "canceled";

export interface UploadEntry {
  id: number;
  file: File;
  name: string;
  size: number;
  loaded: number;
  total: number;
  progress: number;
  status: UploadStatus;
  error: string;
  xhr: XMLHttpRequest | null;
}

const state = reactive({
  entries: [] as UploadEntry[],
  sequence: 0,
  active: 0,
  concurrency: 1,
  collapsed: false,
  visible: false,
});

let initialized = false;

async function initialize() {
  if (initialized) return;
  initialized = true;
  try {
    state.concurrency = (await loadRuntimeConfig()).uploadConcurrency;
  } catch {
    state.concurrency = 1;
  }
}

function processQueue() {
  void initialize().then(() => {
    while (state.active < state.concurrency) {
      const next = state.entries.find((entry) => entry.status === "queued");
      if (!next) return;
      start(next);
    }
  });
}

function start(upload: UploadEntry) {
  upload.status = "uploading";
  upload.error = "";
  upload.loaded = 0;
  upload.progress = 0;
  state.active += 1;

  const form = new FormData();
  form.append("file", upload.file, upload.name);
  const xhr = new XMLHttpRequest();
  upload.xhr = xhr;

  xhr.upload.onprogress = (event) => {
    if (!event.lengthComputable) return;
    upload.loaded = event.loaded;
    upload.total = event.total || upload.size;
    upload.progress = Math.min(
      99,
      Math.round((event.loaded / upload.total) * 100),
    );
  };
  xhr.onload = () => {
    if (xhr.status >= 200 && xhr.status < 300) {
      upload.loaded = upload.total;
      upload.progress = 100;
      upload.status = "done";
      try {
        const item = JSON.parse(xhr.responseText) as Item;
        if (Number.isSafeInteger(item.id) && item.id > 0)
          publishRealtime({ type: "item:result", id: item.id, item });
      } catch {
        publishRealtime({ type: "reconcile" });
      }
    } else {
      upload.status = "failed";
      try {
        upload.error =
          (JSON.parse(xhr.responseText) as { message?: string }).message ??
          `Upload failed with HTTP ${xhr.status}`;
      } catch {
        upload.error = `Upload failed with HTTP ${xhr.status}`;
      }
    }
    finish(upload);
  };
  xhr.onerror = () => {
    upload.status = "failed";
    upload.error = "Network error";
    finish(upload);
  };
  xhr.onabort = () => {
    upload.status = "canceled";
    upload.error = "";
    finish(upload);
  };
  xhr.open("POST", "/api/upload");
  xhr.setRequestHeader("Accept", "application/json");
  xhr.send(form);
}

function finish(upload: UploadEntry) {
  upload.xhr = null;
  state.active = Math.max(0, state.active - 1);
  processQueue();
}

function enqueue(files: File[]) {
  for (const file of files) {
    state.entries.push({
      id: ++state.sequence,
      file,
      name: file.name,
      size: file.size,
      loaded: 0,
      total: file.size,
      progress: 0,
      status: "queued",
      error: "",
      xhr: null,
    });
  }
  if (files.length) {
    state.visible = true;
    state.collapsed = false;
    processQueue();
  }
}

function cancel(upload: UploadEntry) {
  if (upload.status === "queued") upload.status = "canceled";
  else upload.xhr?.abort();
}

function retry(upload: UploadEntry) {
  if (upload.status !== "failed" && upload.status !== "canceled") return;
  upload.loaded = 0;
  upload.progress = 0;
  upload.error = "";
  upload.status = "queued";
  processQueue();
}

function clearFinished() {
  state.entries = state.entries.filter(
    (entry) => !["done", "failed", "canceled"].includes(entry.status),
  );
  if (!state.entries.length) state.visible = false;
}

function close() {
  for (const entry of state.entries) {
    if (entry.status === "queued" || entry.status === "uploading")
      cancel(entry);
  }
  state.entries = [];
  state.visible = false;
}

export function useUploads() {
  return {
    state,
    enqueue,
    cancel,
    retry,
    clearFinished,
    close,
    hasFinished: computed(() =>
      state.entries.some((entry) =>
        ["done", "failed", "canceled"].includes(entry.status),
      ),
    ),
    totalBytes: computed(() =>
      state.entries.reduce((sum, entry) => sum + entry.total, 0),
    ),
    uploadedBytes: computed(() =>
      state.entries.reduce((sum, entry) => sum + entry.loaded, 0),
    ),
    overallProgress: computed(() => {
      const total = state.entries.reduce((sum, entry) => sum + entry.total, 0);
      return total
        ? Math.round(
            (state.entries.reduce((sum, entry) => sum + entry.loaded, 0) /
              total) *
              100,
          )
        : 0;
    }),
  };
}

export function uploadErrorMessage(error: unknown) {
  return error instanceof APIError ? error.message : "Upload failed";
}
