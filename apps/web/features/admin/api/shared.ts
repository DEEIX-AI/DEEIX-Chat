import type { PagePayload } from "@/shared/api/common.types";

export type AdminPageOptions = {
  page?: number;
  pageSize?: number;
};

export type AdminListQueryOptions = AdminPageOptions & {
  query?: string;
  status?: string;
  type?: string;
  pinned?: string;
  sort?: string;
};

export function resolveAdminPage(options: AdminPageOptions = {}): {
  page: number;
  pageSize: number;
} {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 25;

  return { page, pageSize };
}

export function normalizeAdminPagePayload<T>(data: PagePayload<T>): PagePayload<T> {
  return {
    total: data.total ?? 0,
    results: data.results ?? [],
  };
}

export async function listAllAdminPages<T>(
  loader: (options: Required<AdminPageOptions>) => Promise<PagePayload<T>>,
  pageSize = 1000,
): Promise<T[]> {
  const safePageSize = Math.min(Math.max(1, pageSize), 1000);
  const first = await loader({ page: 1, pageSize: safePageSize });
  const results = [...first.results];
  const total = first.total ?? results.length;
  for (let page = 2; results.length < total; page += 1) {
    const next = await loader({ page, pageSize: safePageSize });
    if (next.results.length === 0) {
      break;
    }
    results.push(...next.results);
  }
  return results;
}
