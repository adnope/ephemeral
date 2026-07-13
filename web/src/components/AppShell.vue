<script setup lang="ts">
import { LogOut, Menu, MessageSquareText, Search, X } from "@lucide/vue";
import { ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { api } from "../api";

defineProps<{ selectionActive?: boolean }>();
const emit = defineEmits<{ toggleSelection: [] }>();
const router = useRouter();
const mobileOpen = ref(false);

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
          <button
            class="nav-link"
            :class="{ 'router-link-active': selectionActive }"
            type="button"
            @click="emit('toggleSelection')"
          >
            Select
          </button>
        </nav>

        <div class="header-spacer" />
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
