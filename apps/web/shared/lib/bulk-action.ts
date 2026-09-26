import { toast } from "sonner";

export const DEFAULT_BULK_ACTION_CHUNK_SIZE = 100;

type BulkProgress = {
  chunkIndex: number;
  chunkTotal: number;
  processed: number;
  total: number;
};

type RunBulkActionInChunksArgs<TItem, TResult> = {
  chunkSize?: number;
  describeProgress?: (progress: BulkProgress) => string;
  items: TItem[];
  runChunk: (chunk: TItem[], progress: BulkProgress) => Promise<TResult>;
  title: string;
};

export type SettledBulkItemResult<TItem, TValue> =
  | {
      item: TItem;
      status: "fulfilled";
      value: TValue;
    }
  | {
      item: TItem;
      reason: unknown;
      status: "rejected";
    };

type RunSettledBulkItemsArgs<TItem, TValue> = {
  chunkSize?: number;
  describeProgress?: (progress: BulkProgress) => string;
  items: TItem[];
  runItem: (item: TItem) => Promise<TValue>;
  title: string;
};

type RunSettledItemsWithConcurrencyArgs<TItem, TValue> = {
  concurrency?: number;
  items: TItem[];
  runItem: (item: TItem) => Promise<TValue>;
  signal?: AbortSignal;
};

type BatchResultData<TResult> = {
  total: number;
  successCount: number;
  notFoundCount: number;
  failedCount: number;
  results: TResult[];
};

function chunkItems<TItem>(items: TItem[], chunkSize: number): TItem[][] {
  const resolvedChunkSize = Math.max(1, Math.floor(chunkSize));
  const chunks: TItem[][] = [];
  for (let index = 0; index < items.length; index += resolvedChunkSize) {
    chunks.push(items.slice(index, index + resolvedChunkSize));
  }
  return chunks;
}

function defaultProgressDescription({ processed, total }: BulkProgress): string {
  return `${processed} / ${total}`;
}

export async function runBulkActionInChunks<TItem, TResult>({
  chunkSize = DEFAULT_BULK_ACTION_CHUNK_SIZE,
  describeProgress = defaultProgressDescription,
  items,
  runChunk,
  title,
}: RunBulkActionInChunksArgs<TItem, TResult>): Promise<TResult[]> {
  if (items.length === 0) {
    return [];
  }

  const chunks = chunkItems(items, chunkSize);
  const toastID = chunks.length > 1 ? toast.loading(title, {
    description: describeProgress({
      chunkIndex: 0,
      chunkTotal: chunks.length,
      processed: 0,
      total: items.length,
    }),
  }) : undefined;
  const results: TResult[] = [];
  let processed = 0;

  try {
    for (let index = 0; index < chunks.length; index += 1) {
      const chunk = chunks[index] ?? [];
      const progress = {
        chunkIndex: index + 1,
        chunkTotal: chunks.length,
        processed,
        total: items.length,
      };
      const result = await runChunk(chunk, progress);
      results.push(result);
      processed += chunk.length;
      if (toastID !== undefined) {
        toast.loading(title, {
          id: toastID,
          description: describeProgress({
            ...progress,
            processed,
          }),
        });
      }
    }
    return results;
  } finally {
    if (toastID !== undefined) {
      toast.dismiss(toastID);
    }
  }
}

export function mergeBatchResultData<TResult>(
  parts: Array<BatchResultData<TResult>>,
): BatchResultData<TResult> {
  return parts.reduce<BatchResultData<TResult>>(
    (merged, part) => ({
      total: merged.total + part.total,
      successCount: merged.successCount + part.successCount,
      notFoundCount: merged.notFoundCount + part.notFoundCount,
      failedCount: merged.failedCount + part.failedCount,
      results: [...merged.results, ...part.results],
    }),
    {
      total: 0,
      successCount: 0,
      notFoundCount: 0,
      failedCount: 0,
      results: [],
    },
  );
}

export async function runSettledBulkItems<TItem, TValue>({
  chunkSize = 10,
  describeProgress,
  items,
  runItem,
  title,
}: RunSettledBulkItemsArgs<TItem, TValue>): Promise<Array<SettledBulkItemResult<TItem, TValue>>> {
  const chunks = await runBulkActionInChunks({
    chunkSize,
    describeProgress,
    items,
    title,
    runChunk: async (chunk) => {
      const results: Array<SettledBulkItemResult<TItem, TValue>> = [];
      for (const item of chunk) {
        try {
          results.push({
            item,
            status: "fulfilled",
            value: await runItem(item),
          });
        } catch (reason) {
          results.push({
            item,
            reason,
            status: "rejected",
          });
        }
      }
      return results;
    },
  });

  return chunks.flat();
}

export async function runSettledItemsWithConcurrency<TItem, TValue>({
  concurrency = 4,
  items,
  runItem,
  signal,
}: RunSettledItemsWithConcurrencyArgs<TItem, TValue>): Promise<Array<SettledBulkItemResult<TItem, TValue>>> {
  const results: Array<SettledBulkItemResult<TItem, TValue> | undefined> = new Array(items.length);
  let nextIndex = 0;
  const workerCount = Math.min(Math.max(1, Math.floor(concurrency)), items.length);

  await Promise.all(Array.from({ length: workerCount }, async () => {
    while (!signal?.aborted && nextIndex < items.length) {
      const index = nextIndex;
      nextIndex += 1;
      const item = items[index];
      try {
        results[index] = {
          item,
          status: "fulfilled",
          value: await runItem(item),
        };
      } catch (reason) {
        if (!signal?.aborted) {
          results[index] = { item, reason, status: "rejected" };
        }
      }
    }
  }));

  return results.filter((result): result is SettledBulkItemResult<TItem, TValue> => result !== undefined);
}
