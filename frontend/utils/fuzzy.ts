/** Higher is a better match. 0 means no match. Substring hits outrank subsequence hits. */
export function fuzzyScore(query: string, text: string): number {
  const qRaw = query.trim().toLowerCase();
  if (!qRaw) return 1;
  const hay = text.toLowerCase();
  const at = hay.indexOf(qRaw);
  if (at === 0) return 2000 - hay.length;
  if (at > 0) return 1500 - at;

  const q = qRaw.replace(/[^a-z0-9]/g, "");
  const h = hay.replace(/[^a-z0-9]/g, "");
  if (!q) return 0;
  let qi = 0;
  let score = 0;
  let streak = 0;
  let first = -1;
  for (let i = 0; i < h.length && qi < q.length; i++) {
    if (h[i] !== q[qi]) {
      streak = 0;
      continue;
    }
    if (first < 0) first = i;
    streak++;
    score += 4 + streak * 3;
    qi++;
  }
  if (qi < q.length) return 0;
  return Math.max(1, score - first);
}

export function fuzzyRank<T>(query: string, items: T[], text: (item: T) => string): T[] {
  return items
    .map((item) => ({ item, score: fuzzyScore(query, text(item)) }))
    .filter((row) => row.score > 0)
    .sort(
      (a, b) => b.score - a.score || text(a.item).localeCompare(text(b.item)),
    )
    .map((row) => row.item);
}
