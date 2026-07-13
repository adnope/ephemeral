<script setup lang="ts">
import { LockKeyhole, UserRound } from "@lucide/vue";
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api";

const router = useRouter();
const username = ref("");
const password = ref("");
const setupRequired = ref(false);
const loading = ref(true);
const submitting = ref(false);
const error = ref("");

onMounted(async () => {
  try {
    setupRequired.value = (await api.authState()).setupRequired;
  } catch {
    error.value = "Could not load authentication state";
  } finally {
    loading.value = false;
  }
});

async function submit() {
  error.value = "";
  submitting.value = true;
  try {
    await api.login(username.value, password.value);
    await router.replace("/");
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Login failed";
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-card">
      <h1>Ephemeral</h1>
      <p v-if="setupRequired" class="muted">
        Choose the credentials for this installation.
      </p>
      <div v-if="loading" class="loading-block">Loading...</div>
      <form v-else class="auth-form" @submit.prevent="submit">
        <label
          ><span>Username</span
          ><span class="input-wrap"
            ><UserRound :size="18" /><input
              v-model="username"
              autocomplete="username"
              required
              autofocus /></span
        ></label>
        <label
          ><span>Password</span
          ><span class="input-wrap"
            ><LockKeyhole :size="18" /><input
              v-model="password"
              type="password"
              :autocomplete="
                setupRequired ? 'new-password' : 'current-password'
              "
              required /></span
        ></label>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <button
          class="button button-primary auth-submit"
          type="submit"
          :disabled="submitting"
        >
          {{
            submitting
              ? "Please wait..."
              : setupRequired
                ? "Create account"
                : "Sign in"
          }}
        </button>
      </form>
    </section>
  </main>
</template>
