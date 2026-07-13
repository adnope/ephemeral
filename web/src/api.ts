import type {
  APIErrorBody,
  FilePreview,
  HistoryFilters,
  Item,
  ItemPage,
  PublicLink,
  PublicLinkStatus,
  PublicShare,
  RuntimeConfig,
} from "./types";

export class APIError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "APIError";
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (
    init.body &&
    !(init.body instanceof FormData) &&
    !headers.has("Content-Type")
  ) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(path, { ...init, headers });
  if (!response.ok) {
    let body: APIErrorBody | null = null;
    try {
      body = (await response.json()) as APIErrorBody;
    } catch {
      // Keep the HTTP status fallback when the response body is not JSON.
    }
    const error = new APIError(
      response.status,
      body?.code ?? "request_failed",
      body?.message ?? `Request failed with HTTP ${response.status}`,
    );
    if (
      response.status === 401 &&
      globalThis.location.pathname !== "/login" &&
      !globalThis.location.pathname.startsWith("/share/")
    ) {
      globalThis.location.assign("/login");
    }
    throw error;
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export const api = {
  authState: () => request<{ setupRequired: boolean }>("/api/auth/state"),
  login: (username: string, password: string) =>
    request<{ authenticated: boolean }>("/api/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<void>("/api/logout", { method: "POST" }),
  config: () => request<RuntimeConfig>("/api/config"),
  items: (cursor = 0) =>
    request<ItemPage>(`/api/items${cursor ? `?cursor=${cursor}` : ""}`),
  item: (id: number) => request<Item>(`/api/items/${id}`),
  history: (filters: HistoryFilters, cursor = 0) => {
    const params = new URLSearchParams();
    if (cursor) params.set("cursor", String(cursor));
    if (filters.type) params.set("type", filters.type);
    if (filters.q) params.set("q", filters.q);
    if (filters.body) params.set("body", "1");
    if (filters.from) params.set("from", filters.from);
    if (filters.to) params.set("to", filters.to);
    if (filters.recent) params.set("recent", filters.recent);
    if (filters.visibility) params.set("visibility", filters.visibility);
    return request<ItemPage>(`/api/history?${params.toString()}`);
  },
  message: (text: string) =>
    request<Item>("/api/message", {
      method: "POST",
      body: JSON.stringify({ text }),
    }),
  deleteItem: (id: number) =>
    request<void>(`/api/items/${id}`, { method: "DELETE" }),
  preview: (id: number) => request<FilePreview>(`/api/file-preview/${id}`),
  publicLinkStatus: (id: number) =>
    request<PublicLinkStatus>(`/api/items/${id}/public-link`),
  createPublicLink: (id: number, expiresInSeconds: number | null) =>
    request<PublicLink>(`/api/items/${id}/public-link`, {
      method: "POST",
      body: JSON.stringify({ expires_in_seconds: expiresInSeconds }),
    }),
  revokePublicLink: (id: number) =>
    request<void>(`/api/items/${id}/public-link`, { method: "DELETE" }),
  publicShare: (token: string) =>
    request<PublicShare>(`/api/share/${encodeURIComponent(token)}`),
};
