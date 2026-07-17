<script setup lang="ts">
import {
  LogOut,
  Menu,
  MessageSquareText,
  Moon,
  Search,
  Sun,
  X,
} from "@lucide/vue";
import { computed, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { api } from "../api";
import { useTheme } from "../state/theme";

const router = useRouter();
const mobileOpen = ref(false);
const { isLight, toggleTheme } = useTheme();
const themeLabel = computed(() =>
  isLight.value ? "Switch to dark mode" : "Switch to light mode",
);

async function logout() {
  await api.logout();
  await router.replace("/login");
}
</script>

<template>
  <div class="app-frame">
    <header class="app-header">
      <div class="header-inner">
        <RouterLink class="brand" to="/" aria-label="Ephemeral Chat">
          <span>Ephemeral</span>
        </RouterLink>

        <button
          class="icon-button mobile-menu-button"
          type="button"
          aria-label="Toggle navigation"
          @click="mobileOpen = !mobileOpen"
        >
          <X v-if="mobileOpen" :size="22" />
          <Menu v-else :size="22" />
        </button>

        <nav
          class="primary-nav"
          :class="{ 'is-open': mobileOpen }"
          aria-label="Primary navigation"
        >
          <RouterLink to="/" class="nav-link" @click="mobileOpen = false">
            <MessageSquareText :size="17" /> Chat
          </RouterLink>
          <RouterLink
            to="/history"
            class="nav-link"
            @click="mobileOpen = false"
          >
            <Search :size="17" /> History
          </RouterLink>
        </nav>

        <div class="header-spacer" />
        <button
          class="button button-outline theme-toggle-button"
          type="button"
          :aria-label="themeLabel"
          :aria-pressed="isLight"
          :title="themeLabel"
          @click="toggleTheme"
        >
          <Moon v-if="isLight" :size="16" />
          <Sun v-else :size="16" />
          <span>{{ isLight ? "Dark mode" : "Light mode" }}</span>
        </button>
        <button
          class="button button-outline logout-button"
          type="button"
          @click="logout"
        >
          <LogOut :size="16" /> Logout
        </button>
      </div>
    </header>
    <main class="app-main">
      <slot />
    </main>
  </div>
</template>
