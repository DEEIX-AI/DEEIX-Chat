import type { InternalMessagingUserPage } from "../../../shared/api/internal-messaging.types";

export async function loadUserPages(
  fetchPage: (page: number, signal: AbortSignal) => Promise<InternalMessagingUserPage>,
  count: number,
  signal: AbortSignal,
) {
  let snapshot: InternalMessagingUserPage | undefined;
  let loaded = 0;
  for (let page = 1; page <= count; page++) {
    signal.throwIfAborted();
    const result = await fetchPage(page, signal);
    signal.throwIfAborted();
    const byID = new Map(snapshot?.results.map((item) => [item.publicID, item]));
    for (const item of result.results) byID.set(item.publicID, item);
    snapshot = { ...result, results: [...byID.values()] };
    loaded = page;
    if (!result.hasMore) break;
  }
  return { snapshot, loaded };
}
