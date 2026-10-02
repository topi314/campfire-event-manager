/** Absolute URL for img/src of a stored cover or external https cover. */
export function coverImageSrc(url: string | null | undefined, apiBase = ""): string {
  const raw = (url || "").trim();
  if (!raw) return "";
  if (raw.startsWith("/api/")) {
    return `${apiBase || ""}${raw}`;
  }
  return raw;
}

/** Numeric id from `/api/covers/{id}` (relative or absolute). */
export function parseCoverImageId(url: string | null | undefined): number | null {
  const raw = (url || "").trim();
  const m = raw.match(/\/api\/covers\/(\d+)(?:[/?#]|$)/);
  if (!m) return null;
  const id = Number(m[1]);
  return Number.isFinite(id) && id > 0 ? id : null;
}
