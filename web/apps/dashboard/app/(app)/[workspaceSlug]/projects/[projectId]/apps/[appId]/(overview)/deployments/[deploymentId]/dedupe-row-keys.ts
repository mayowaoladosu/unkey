export function dedupeRowsByKey<T extends { key: string }>(
  rows: readonly T[],
): T[] {
  const seen = new Set<string>();

  return rows.filter((row) => {
    if (seen.has(row.key)) {
      return false;
    }
    seen.add(row.key);
    return true;
  });
}
