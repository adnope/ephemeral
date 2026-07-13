<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import { RouterView, useRoute } from "vue-router";
import UploadQueue from "./components/uploads/UploadQueue.vue";
import { ui } from "./state/ui";

const DeleteDialog = defineAsyncComponent(
  () => import("./components/dialogs/DeleteDialog.vue"),
);
const MediaDialog = defineAsyncComponent(
  () => import("./components/dialogs/MediaDialog.vue"),
);
const PreviewDialog = defineAsyncComponent(
  () => import("./components/dialogs/PreviewDialog.vue"),
);
const ShareDialog = defineAsyncComponent(
  () => import("./components/dialogs/ShareDialog.vue"),
);

const route = useRoute();
const showApplicationOverlays = computed(() => !route.meta.public);
</script>

<template>
  <RouterView />
  <template v-if="showApplicationOverlays">
    <UploadQueue />
    <DeleteDialog v-if="ui.deleteItems.length" />
    <ShareDialog v-if="ui.shareItem" />
    <PreviewDialog v-if="ui.previewItem" />
    <MediaDialog v-if="ui.mediaItem" />
  </template>
</template>
