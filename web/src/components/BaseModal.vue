<script setup lang="ts">
import { X } from "@lucide/vue";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    description?: string;
    wide?: boolean;
    media?: boolean;
    closeOnBackdrop?: boolean;
  }>(),
  { description: "", wide: false, media: false, closeOnBackdrop: true },
);
const emit = defineEmits<{ close: [] }>();
const panel = ref<HTMLElement | null>(null);
let previousFocus: HTMLElement | null = null;

function close() {
  emit("close");
}

function onKeydown(event: KeyboardEvent) {
  if (!props.open) return;
  if (event.key === "Escape") close();
  if (event.key !== "Tab" || !panel.value) return;
  const focusable = [
    ...panel.value.querySelectorAll<HTMLElement>(
      'button, a, input, select, textarea, [tabindex]:not([tabindex="-1"])',
    ),
  ].filter((element) => !element.hasAttribute("disabled"));
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last?.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first?.focus();
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      previousFocus = document.activeElement as HTMLElement | null;
      document.body.classList.add("modal-open");
      requestAnimationFrame(() => panel.value?.focus());
    } else {
      document.body.classList.remove("modal-open");
      previousFocus?.focus();
    }
  },
  { immediate: true },
);

onMounted(() => document.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeydown);
  document.body.classList.remove("modal-open");
});
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="modal-backdrop"
        role="presentation"
        @mousedown.self="closeOnBackdrop && close()"
      >
        <section
          ref="panel"
          class="modal-panel"
          :class="{ 'is-wide': wide, 'is-media': media }"
          role="dialog"
          aria-modal="true"
          :aria-label="title"
          tabindex="-1"
        >
          <header class="modal-header">
            <div>
              <slot name="title">
                <h2>{{ title }}</h2>
              </slot>
              <p v-if="description" class="muted">{{ description }}</p>
            </div>
            <button
              class="icon-button"
              type="button"
              aria-label="Close"
              @click="close"
            >
              <X :size="20" />
            </button>
          </header>
          <div class="modal-content"><slot /></div>
          <footer v-if="$slots.footer" class="modal-footer">
            <slot name="footer" />
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
