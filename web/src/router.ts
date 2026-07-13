import { createRouter, createWebHistory } from "vue-router";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "chat",
      component: () => import("./views/ChatView.vue"),
    },
    {
      path: "/history",
      name: "history",
      component: () => import("./views/HistoryView.vue"),
    },
    {
      path: "/login",
      name: "login",
      component: () => import("./views/LoginView.vue"),
      meta: { public: true },
    },
    {
      path: "/share/:token",
      name: "public-share",
      component: () => import("./views/PublicShareView.vue"),
      meta: { public: true },
    },
  ],
});

router.afterEach((to) => {
  document.title = to.name === "history" ? "Ephemeral - History" : "Ephemeral";
});

export default router;
