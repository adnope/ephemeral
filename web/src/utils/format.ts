export function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return unit === 0
    ? `${Math.round(value)} B`
    : `${value.toFixed(1)} ${units[unit]}`;
}

export function formatDateTime(epochMillis: number) {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(epochMillis));
}

export function formatFullDateTime(epochMillis: number) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(new Date(epochMillis));
}

export function linkify(text: string) {
  const pattern =
    /(?:https?:\/\/|www\.)[^\s<]+|(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?:\/[^\s<]*)?/gi;
  const segments: Array<{ text: string; href?: string }> = [];
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > last) segments.push({ text: text.slice(last, index) });
    let raw = match[0];
    let trailing = "";
    while (raw && ".,!?:;)]}".includes(raw.at(-1) ?? "")) {
      trailing = raw.at(-1) + trailing;
      raw = raw.slice(0, -1);
    }
    segments.push({
      text: raw,
      href: /^https?:\/\//i.test(raw) ? raw : `https://${raw}`,
    });
    if (trailing) segments.push({ text: trailing });
    last = index + match[0].length;
  }
  if (last < text.length) segments.push({ text: text.slice(last) });
  return segments;
}
