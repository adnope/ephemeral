import { computed, readonly, ref } from "vue";

export type Theme = "dark" | "light";

const STORAGE_KEY = "ephemeral-theme";
const theme = ref<Theme>(readTheme());

applyTheme(theme.value);

function readTheme(): Theme {
  if (typeof window === "undefined") return "dark";

  try {
    return window.localStorage.getItem(STORAGE_KEY) === "light"
      ? "light"
      : "dark";
  } catch {
    return "dark";
  }
}

function applyTheme(value: Theme) {
  if (typeof document === "undefined") return;

  document.documentElement.dataset.theme = value;
  document
    .querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    ?.setAttribute("content", value === "light" ? "#f8fafc" : "#111111");
}

function setTheme(value: Theme) {
  theme.value = value;
  applyTheme(value);

  try {
    window.localStorage.setItem(STORAGE_KEY, value);
  } catch {
    // Theme changes still work when storage is unavailable.
  }
}

export function useTheme() {
  return {
    theme: readonly(theme),
    isLight: computed(() => theme.value === "light"),
    toggleTheme: () => setTheme(theme.value === "light" ? "dark" : "light"),
  };
}
