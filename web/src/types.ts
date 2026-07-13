export type ItemType = "text" | "image" | "video" | "file";

export interface ItemMetadata {
  width: number;
  height: number;
  duration: string;
  mime: string;
  thumbnailUrl: string;
  playbackUrl: string;
  playbackMime: string;
  hlsUrl: string;
  processing: boolean;
}

export interface Item {
  id: number;
  type: ItemType;
  text: string;
  filename: string;
  filesizeBytes: number;
  contentUrl: string;
  downloadUrl: string;
  createdAtEpochMillis: number;
  publicLinkActive: boolean;
  metadata: ItemMetadata;
}

export interface ItemPage {
  items: Item[];
  nextCursor: number;
  hasMore: boolean;
}

export interface RuntimeConfig {
  chatPageSize: number;
  historyPageSize: number;
  maxUploadSizeBytes: number;
  textPreviewMaxBytes: number;
  uploadConcurrency: number;
}

export interface APIErrorBody {
  code: string;
  message: string;
}

export interface FilePreview {
  id: number;
  filename: string;
  mime: string;
  language: string;
  content: string;
  filesize: number;
  created_at: string;
  download_url: string;
}

export interface PublicLinkStatus {
  status: "none" | "active" | "expired";
  url?: string;
  token?: string;
  expires_at: string | null;
}

export interface PublicLink {
  url: string;
  token: string;
  expires_at: string | null;
}

export interface PublicShare {
  filename: string;
  filesizeBytes: number;
  itemType: ItemType;
  mime: string;
  sourceUrl: string;
  posterUrl: string;
  downloadUrl: string;
  expiresAt: string | null;
  processing: boolean;
}

export interface HistoryFilters {
  type: string;
  q: string;
  body: boolean;
  from: string;
  to: string;
  recent: string;
  visibility: string;
}
