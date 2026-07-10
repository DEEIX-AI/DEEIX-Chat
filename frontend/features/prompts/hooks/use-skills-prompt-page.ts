"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  createMyPromptPreset,
  deleteMyPromptPreset,
  listMyPromptPresets,
  listVisiblePromptPresets,
  updateMyPromptPreset,
} from "@/shared/api/prompt-presets";
import type {
  PatchPromptPresetRequest,
  PromptPresetDTO,
  WritePromptPresetRequest,
} from "@/shared/api/prompt-presets.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { PROMPT_PRESET_LIMITS, normalizePromptPresetName } from "@/shared/model/prompt-presets";

const PROMPT_PRESET_PAGE_SIZE = 100;
const PROMPT_PRESET_SEARCH_DEBOUNCE_MS = 250;

export type PromptPresetForm = {
  id?: number;
  name: string;
  description: string;
  content: string;
  enabled: boolean;
};

type PromptPresetSourcePage = {
  nextPage: number;
  hasMore: boolean;
};

type PromptPresetPagination = {
  mine: PromptPresetSourcePage;
  visible: PromptPresetSourcePage;
};

type PromptPresetPageLoad = {
  items: PromptPresetDTO[];
  pagination: PromptPresetPagination;
};

export const emptyPromptPresetForm: PromptPresetForm = {
  name: "",
  description: "",
  content: "",
  enabled: true,
};

const initialPagination: PromptPresetPagination = {
  mine: { nextPage: 1, hasMore: true },
  visible: { nextPage: 1, hasMore: true },
};

function formFromPromptPreset(item: PromptPresetDTO): PromptPresetForm {
  return {
    id: item.id,
    name: item.trigger,
    description: item.description,
    content: item.content,
    enabled: item.enabled,
  };
}

function payloadFromForm(form: PromptPresetForm): WritePromptPresetRequest {
  const name = normalizePromptPresetName(form.name);
  return {
    title: name,
    trigger: name,
    description: form.description.trim(),
    content: form.content.trim(),
    enabled: form.enabled,
    sortOrder: 0,
  };
}

function formExceedsLimits(form: PromptPresetForm): boolean {
  return (
    normalizePromptPresetName(form.name).length > PROMPT_PRESET_LIMITS.name ||
    form.description.trim().length > PROMPT_PRESET_LIMITS.description ||
    form.content.trim().length > PROMPT_PRESET_LIMITS.content
  );
}

export function promptMatchesQuery(item: PromptPresetDTO, query: string): boolean {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return [
    item.title,
    item.trigger,
    item.description,
    item.content,
  ].some((value) => value.toLowerCase().includes(normalizedQuery));
}

export function promptPresetKey(item: PromptPresetDTO): string {
  return `${item.scope}-${item.id}`;
}

function mergePromptPresets(current: PromptPresetDTO[], next: PromptPresetDTO[]): PromptPresetDTO[] {
  const seen = new Set(current.map(promptPresetKey));
  const merged = [...current];
  for (const item of next) {
    const key = promptPresetKey(item);
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    merged.push(item);
  }
  return orderPromptPresets(merged);
}

function orderPromptPresets(items: PromptPresetDTO[]): PromptPresetDTO[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((a, b) => {
      const rank = (item: PromptPresetDTO) => {
        if (!item.enabled) {
          return 2;
        }
        return item.scope === "builtin" ? 1 : 0;
      };
      return rank(a.item) - rank(b.item) || a.index - b.index;
    })
    .map(({ item }) => item);
}

function nextSourcePage(total: number, page: number): PromptPresetSourcePage {
  return {
    nextPage: page + 1,
    hasMore: page * PROMPT_PRESET_PAGE_SIZE < total,
  };
}

export function useSkillsPromptPage() {
  const t = useTranslations("prompts");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<PromptPresetDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [loadMoreFailed, setLoadMoreFailed] = React.useState(false);
  const [pagination, setPagination] = React.useState<PromptPresetPagination>(initialPagination);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<PromptPresetForm>(emptyPromptPresetForm);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [viewTarget, setViewTarget] = React.useState<PromptPresetDTO | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<PromptPresetDTO | null>(null);
  const [query, setQuery] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const loadingMoreRef = React.useRef(false);
  const loadMoreFailedRef = React.useRef(false);

  const filteredItems = React.useMemo(() => {
    return orderPromptPresets(items.filter((item) => promptMatchesQuery(item, query)));
  }, [items, query]);

  const hasMore = pagination.mine.hasMore || pagination.visible.hasMore;

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, PROMPT_PRESET_SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [query]);

  React.useEffect(() => {
    loadingMoreRef.current = loadingMore;
  }, [loadingMore]);

  React.useEffect(() => {
    loadMoreFailedRef.current = loadMoreFailed;
  }, [loadMoreFailed]);

  const fetchPage = React.useCallback(
    async (target: PromptPresetPagination): Promise<PromptPresetPageLoad> => {
      const token = await resolveAccessToken();
      if (!token) {
        return {
          items: [],
          pagination: {
            mine: { nextPage: 1, hasMore: false },
            visible: { nextPage: 1, hasMore: false },
          },
        };
      }

      const [mine, visible] = await Promise.all([
        target.mine.hasMore
          ? listMyPromptPresets(token, { page: target.mine.nextPage, pageSize: PROMPT_PRESET_PAGE_SIZE, query: debouncedQuery })
          : Promise.resolve(null),
        target.visible.hasMore
          ? listVisiblePromptPresets(token, { page: target.visible.nextPage, pageSize: PROMPT_PRESET_PAGE_SIZE, query: debouncedQuery })
          : Promise.resolve(null),
      ]);
      const visibleBuiltin = visible?.results.filter((item) => item.scope === "builtin") ?? [];
      const nextItems = [...(mine?.results ?? []), ...visibleBuiltin];

      return {
        items: nextItems,
        pagination: {
          mine: mine ? nextSourcePage(mine.total, target.mine.nextPage) : target.mine,
          visible: visible ? nextSourcePage(visible.total, target.visible.nextPage) : target.visible,
        },
      };
    },
    [debouncedQuery],
  );

  React.useEffect(() => {
    let cancelled = false;
    async function loadInitial() {
      setLoading(true);
      setLoadMoreFailed(false);
      loadMoreFailedRef.current = false;
      try {
        const data = await fetchPage(initialPagination);
        if (!cancelled) {
          setItems(orderPromptPresets(data.items));
          setPagination(data.pagination);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(t("loadFailed"), { description: resolveErrorMessage(error) });
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void loadInitial();
    return () => {
      cancelled = true;
    };
  }, [fetchPage, resolveErrorMessage, t]);

  const loadMore = React.useCallback(async () => {
    if (loading || loadingMoreRef.current || !hasMore || loadMoreFailedRef.current) {
      return;
    }

    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const data = await fetchPage(pagination);
      setItems((current) => mergePromptPresets(current, data.items));
      setPagination(data.pagination);
      loadMoreFailedRef.current = false;
      setLoadMoreFailed(false);
    } catch (error) {
      loadMoreFailedRef.current = true;
      setLoadMoreFailed(true);
      toast.error(t("loadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [fetchPage, hasMore, loading, pagination, resolveErrorMessage, t]);

  const loadMoreRef = useLoadMoreSentinel<HTMLDivElement>({
    enabled: hasMore && !loading && !loadingMore && !loadMoreFailed,
    rootMargin: "160px",
    onLoadMore: loadMore,
  });

  const retryLoadMore = React.useCallback(async () => {
    setLoadMoreFailed(false);
    loadMoreFailedRef.current = false;
    await loadMore();
  }, [loadMore]);

  const openCreate = React.useCallback(() => {
    setForm(emptyPromptPresetForm);
    setDialogOpen(true);
  }, []);

  const openPrompt = React.useCallback((item: PromptPresetDTO) => {
    if (item.scope === "user") {
      setForm(formFromPromptPreset(item));
      setDialogOpen(true);
      return;
    }
    setViewTarget(item);
  }, []);

  const save = React.useCallback(async () => {
    const payload = payloadFromForm(form);
    if (!payload.title || !payload.trigger || !payload.content) {
      toast.error(t("invalid"));
      return;
    }
    if (formExceedsLimits(form)) {
      toast.error(t("tooLong"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      if (form.id) {
        const data = await updateMyPromptPreset(token, form.id, payload);
        setItems((current) =>
          orderPromptPresets(
            current.map((item) =>
              promptPresetKey(item) === promptPresetKey(data.promptPreset) ? data.promptPreset : item,
            ),
          ),
        );
        toast.success(t("updated"));
      } else {
        const data = await createMyPromptPreset(token, payload);
        setItems((current) => orderPromptPresets([...current, data.promptPreset]));
        toast.success(t("created"));
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? t("updateFailed") : t("createFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [form, resolveErrorMessage, t]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      await deleteMyPromptPreset(token, target.id);
      setItems((current) => current.filter((item) => promptPresetKey(item) !== promptPresetKey(target)));
      toast.success(t("deleted"));
    } catch (error) {
      toast.error(t("deleteFailed"), { description: resolveErrorMessage(error) });
    }
  }, [deleteTarget, resolveErrorMessage, t]);

  const toggleEnabled = React.useCallback(
    async (item: PromptPresetDTO, enabled: boolean) => {
      if (item.scope !== "user") {
        return;
      }
      const previous = item;
      setItems((current) =>
        orderPromptPresets(
          current.map((row) => (promptPresetKey(row) === promptPresetKey(item) ? { ...row, enabled } : row)),
        ),
      );
      try {
        const token = await resolveAccessToken();
        if (!token) {
          setItems((current) =>
            orderPromptPresets(
              current.map((row) => (promptPresetKey(row) === promptPresetKey(previous) ? previous : row)),
            ),
          );
          return;
        }
        const payload: PatchPromptPresetRequest = { enabled };
        const data = await updateMyPromptPreset(token, item.id, payload);
        setItems((current) =>
          orderPromptPresets(
            current.map((row) =>
              promptPresetKey(row) === promptPresetKey(data.promptPreset) ? data.promptPreset : row,
            ),
          ),
        );
      } catch (error) {
        setItems((current) =>
          orderPromptPresets(
            current.map((row) => (promptPresetKey(row) === promptPresetKey(previous) ? previous : row)),
          ),
        );
        toast.error(t("updateFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [resolveErrorMessage, t],
  );

  return {
    items,
    filteredItems,
    loading,
    loadingMore,
    loadMoreFailed,
    hasMore,
    loadMoreRef,
    query,
    setQuery,
    saving,
    form,
    setForm,
    dialogOpen,
    setDialogOpen,
    viewTarget,
    setViewTarget,
    deleteTarget,
    setDeleteTarget,
    openCreate,
    openPrompt,
    save,
    toggleEnabled,
    confirmDelete,
    retryLoadMore,
  };
}
