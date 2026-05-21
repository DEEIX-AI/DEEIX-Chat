export function replaceByID<T, ID>(
  items: T[],
  id: ID,
  resolveID: (item: T) => ID,
  nextItem: T,
): T[] {
  return items.map((item) => (Object.is(resolveID(item), id) ? nextItem : item));
}

export function patchByID<T, ID>(
  items: T[],
  id: ID,
  resolveID: (item: T) => ID,
  patch: Partial<T> | ((item: T) => T),
): T[] {
  return items.map((item) => {
    if (!Object.is(resolveID(item), id)) {
      return item;
    }
    return typeof patch === "function" ? patch(item) : { ...item, ...patch };
  });
}

export function removeByID<T, ID>(
  items: T[],
  id: ID,
  resolveID: (item: T) => ID,
): T[] {
  return items.filter((item) => !Object.is(resolveID(item), id));
}

export function removeManyByID<T, ID>(
  items: T[],
  ids: Iterable<ID>,
  resolveID: (item: T) => ID,
): T[] {
  const idSet = new Set(ids);
  return items.filter((item) => !idSet.has(resolveID(item)));
}

export function upsertByID<T, ID>(
  items: T[],
  nextItem: T,
  resolveID: (item: T) => ID,
): T[] {
  const id = resolveID(nextItem);
  const index = items.findIndex((item) => Object.is(resolveID(item), id));
  if (index < 0) {
    return [nextItem, ...items];
  }
  const next = [...items];
  next[index] = nextItem;
  return next;
}

export function restoreAt<T, ID>(
  items: T[],
  item: T,
  index: number,
  resolveID: (item: T) => ID,
): T[] {
  const id = resolveID(item);
  const existingIndex = items.findIndex((current) => Object.is(resolveID(current), id));
  if (existingIndex >= 0) {
    const next = [...items];
    next[existingIndex] = item;
    return next;
  }
  const next = [...items];
  next.splice(Math.max(0, Math.min(index, next.length)), 0, item);
  return next;
}

export function restoreManyAt<T, ID>(
  items: T[],
  entries: Array<{ item: T; index: number }>,
  resolveID: (item: T) => ID,
): T[] {
  return [...entries]
    .sort((left, right) => left.index - right.index)
    .reduce((current, entry) => restoreAt(current, entry.item, entry.index, resolveID), items);
}
