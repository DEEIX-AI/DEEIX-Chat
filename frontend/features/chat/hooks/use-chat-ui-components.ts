"use client";

import * as React from "react";

import { listVisibleUIComponents } from "@/shared/api/ui-components";
import type { UIComponentDTO } from "@/shared/api/ui-components.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const MAX_SELECTED_UI_COMPONENTS = 32;
// Device-level preference, like the visual-layout prompt: it describes how the
// user wants replies rendered, not what this conversation is about.
const UI_COMPONENT_SELECTION_STORAGE_KEY = "deeix-chat:ui-components:v1";
const useIsomorphicLayoutEffect = typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

// null means "use the defaults"; an array is an explicit selection.
function readStoredSelection(): number[] | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(UI_COMPONENT_SELECTION_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? parsed.filter((id): id is number => Number.isInteger(id) && id > 0) : null;
  } catch {
    return null;
  }
}

function writeStoredSelection(ids: number[] | null): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    if (ids === null) {
      window.localStorage.removeItem(UI_COMPONENT_SELECTION_STORAGE_KEY);
    } else {
      window.localStorage.setItem(UI_COMPONENT_SELECTION_STORAGE_KEY, JSON.stringify(ids));
    }
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

/**
 * 当前用户可用的交互式组件目录与勾选集。勾选集是设备级偏好（localStorage），跨会话记住；
 * 未显式选择时默认勾选全部已启用的内置组件——组件只影响渲染能力，不像技能那样改变模型行为。
 */
export function useChatUIComponents() {
  const [components, setComponents] = React.useState<UIComponentDTO[] | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [selectedUIComponentIDs, setSelectedUIComponentIDs] = React.useState<number[] | null>(null);

  useIsomorphicLayoutEffect(() => {
    setSelectedUIComponentIDs(readStoredSelection());
  }, []);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    function onStorage(event: StorageEvent) {
      if (event.key === UI_COMPONENT_SELECTION_STORAGE_KEY) {
        setSelectedUIComponentIDs(readStoredSelection());
      }
    }
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token || cancelled) {
          return;
        }
        const page = await listVisibleUIComponents(token, { pageSize: 100 });
        if (!cancelled) {
          setComponents(page.results);
        }
      } catch {
        if (!cancelled) {
          setComponents([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const defaultIDs = React.useMemo(
    () => (components ?? []).filter((component) => component.scope === "builtin" && component.enabled).map((component) => component.id),
    [components],
  );

  // Effective selection: explicit choice, else defaults; always pruned to what
  // is still visible so a deleted component does not linger in the request.
  const effectiveIDs = React.useMemo(() => {
    if (!components) {
      return [];
    }
    const visible = new Set(components.map((component) => component.id));
    const source = selectedUIComponentIDs ?? defaultIDs;
    return source.filter((id) => visible.has(id)).slice(0, MAX_SELECTED_UI_COMPONENTS);
  }, [components, defaultIDs, selectedUIComponentIDs]);

  const setSelected = React.useCallback(
    (ids: number[]) => {
      const next = Array.from(new Set(ids)).slice(0, MAX_SELECTED_UI_COMPONENTS);
      const isDefault = next.length === defaultIDs.length && next.every((id) => defaultIDs.includes(id));
      const stored = isDefault ? null : next;
      writeStoredSelection(stored);
      setSelectedUIComponentIDs(stored);
    },
    [defaultIDs],
  );

  return {
    uiComponents: components,
    uiComponentsLoading: loading,
    defaultUIComponentIDs: defaultIDs,
    effectiveUIComponentIDs: effectiveIDs,
    setSelectedUIComponents: setSelected,
  };
}
