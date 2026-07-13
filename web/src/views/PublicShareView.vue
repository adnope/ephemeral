<script setup lang="ts">
import { Download, MessageSquareText } from "@lucide/vue";
import { nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import Hls from "hls.js";
import { APIError, api } from "../api";
import type { PublicShare } from "../types";
import { formatBytes } from "../utils/format";

const route = useRoute();
const share = ref<PublicShare | null>(null);
const error = ref("");
const video = ref<HTMLVideoElement | null>(null);
let hls: Hls | null = null;

onMounted(async () => {
  try {
    share.value = await api.publicShare(String(route.params.token));
    await nextTick();
    if (
      share.value.itemType === "video" &&
      share.value.mime === "application/vnd.apple.mpegurl" &&
      video.value &&
      Hls.isSupported()
    ) {
      hls = new Hls();
      hls.loadSource(share.value.sourceUrl);
      hls.attachMedia(video.value);
    }
  } catch (caught) {
    if (caught instanceof APIError && caught.code === "unsupported_share") {
      globalThis.location.replace(
        `/share/${encodeURIComponent(String(route.params.token))}/download`,
      );
      return;
    }
    error.value =
      caught instanceof Error ? caught.message : "This link is unavailable";
  }
});

onBeforeUnmount(() => hls?.destroy());
</script>

<template>
  <main class="share-page">
    <header class="share-header">
      <span class="brand-mark"><MessageSquareText :size="22" /></span
      ><strong>Ephemeral</strong>
    </header>
    <section v-if="error" class="share-card empty-state">
      <h1>Link unavailable</h1>
      <p>{{ error }}</p>
    </section>
    <section v-else-if="!share" class="share-card state-panel">
      Loading...
    </section>
    <section v-else class="share-card">
      <div class="share-copy">
        <p class="eyebrow">Shared with you</p>
        <h1>{{ share.filename }}</h1>
        <p class="muted">
          {{ formatBytes(share.filesizeBytes)
          }}<template v-if="share.expiresAt">
            · Expires {{ new Date(share.expiresAt).toLocaleString() }}
          </template>
        </p>
      </div>
      <div class="public-media">
        <img
          v-if="share.itemType === 'image'"
          :src="share.sourceUrl"
          :alt="share.filename"
        />
        <div v-else-if="share.processing" class="processing-panel">
          <span class="spin-ring" />
          <h3>Processing video</h3>
        </div>
        <video
          v-else-if="share.itemType === 'video'"
          ref="video"
          controls
          playsinline
          :poster="share.posterUrl || undefined"
        >
          <source :src="share.sourceUrl" :type="share.mime" />
        </video>
      </div>
      <a class="button button-primary share-download" :href="share.downloadUrl"
        ><Download :size="17" />Download</a
      >
    </section>
  </main>
</template>
