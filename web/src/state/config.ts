import { readonly, ref } from "vue";
import { api } from "../api";
import type { RuntimeConfig } from "../types";

const config = ref<RuntimeConfig | null>(null);
let loading: Promise<RuntimeConfig> | null = null;

export async function loadRuntimeConfig() {
  if (config.value) return config.value;
  loading ??= api.config().then((value) => {
    config.value = value;
    return value;
  });
  return loading;
}

export function useRuntimeConfig() {
  return readonly(config);
}
