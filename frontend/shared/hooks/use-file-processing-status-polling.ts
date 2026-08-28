"use client";

import * as React from "react";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { getFileProcessingStatuses } from "@/shared/api/file";
import type { FileProcessingStatusDTO } from "@/shared/api/file.types";

type FileStatus = {
  fileID: string;
};

export type FileStatusPollingResult<Status extends FileStatus, Snapshot = Status[]> = {
  statuses: Status[];
  missingFileIDs: string[];
  snapshot: Snapshot;
};

type FileStatusPollingOptions<Status extends FileStatus, Snapshot = Status[]> = {
  fileIDs: string[];
  intervalMs: number;
  enabled?: boolean;
  loadStatuses: (accessToken: string, fileIDs: string[], signal: AbortSignal) => Promise<Snapshot>;
  selectStatuses: (snapshot: Snapshot) => Status[];
  onResult: (result: FileStatusPollingResult<Status, Snapshot>) => void;
};

function selectStatusArray<Status extends FileStatus>(statuses: Status[]): Status[] {
  return statuses;
}

export function useFileStatusPolling<Status extends FileStatus, Snapshot = Status[]>({
  fileIDs,
  intervalMs,
  enabled,
  loadStatuses,
  selectStatuses,
  onResult,
}: FileStatusPollingOptions<Status, Snapshot>) {
  const fileIDsKey = Array.from(new Set(fileIDs.filter(Boolean))).sort().join("\u0000");
  const onResultRef = React.useRef(onResult);
  const selectStatusesRef = React.useRef(selectStatuses);

  React.useEffect(() => {
    onResultRef.current = onResult;
    selectStatusesRef.current = selectStatuses;
  }, [onResult, selectStatuses]);

  React.useEffect(() => {
    if (!(enabled ?? Boolean(fileIDsKey))) {
      return;
    }

    let cancelled = false;
    let failureCount = 0;
    let polling = false;
    let timer: number | undefined;
    let requestController: AbortController | null = null;
    const requestedFileIDs = fileIDsKey ? fileIDsKey.split("\u0000") : [];
    const missingObservations = new Map<string, { count: number; firstSeenAt: number }>();
    const schedule = () => {
      if (!cancelled && !document.hidden) {
        timer = window.setTimeout(
          poll,
          intervalMs * Math.min(2 ** failureCount, 4),
        );
      }
    };
    const poll = async () => {
      if (cancelled || polling || document.hidden) {
        return;
      }
      polling = true;
      let snapshot: Snapshot | null = null;
      requestController = new AbortController();
      const controller = requestController;
      try {
        const accessToken = await resolveAccessToken();
        if (!accessToken || cancelled || controller.signal.aborted || document.hidden) {
          return;
        }
        snapshot = await loadStatuses(accessToken, requestedFileIDs, controller.signal);
        failureCount = 0;
      } catch {
        if (!controller.signal.aborted) {
          failureCount += 1;
          // Polling is best-effort; the next cycle retries without interrupting the UI.
        }
      } finally {
        if (requestController === controller) {
          requestController = null;
        }
        polling = false;
        schedule();
      }
      if (!cancelled && snapshot !== null) {
        const statuses = selectStatusesRef.current(snapshot);
        const returnedFileIDs = new Set(statuses.map((status) => status.fileID));
        const observedAt = Date.now();
        const missingFileIDs: string[] = [];
        for (const fileID of requestedFileIDs) {
          if (returnedFileIDs.has(fileID)) {
            missingObservations.delete(fileID);
            continue;
          }
          const previous = missingObservations.get(fileID);
          const observation = previous
            ? { count: previous.count + 1, firstSeenAt: previous.firstSeenAt }
            : { count: 1, firstSeenAt: observedAt };
          missingObservations.set(fileID, observation);
          if (
            observation.count >= 2 &&
            observedAt - observation.firstSeenAt >= Math.max(intervalMs * 2, 3000)
          ) {
            missingFileIDs.push(fileID);
          }
        }
        onResultRef.current({ statuses, missingFileIDs, snapshot });
      }
    };
    const handleVisibilityChange = () => {
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
      if (document.hidden) {
        requestController?.abort();
      } else {
        void poll();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    void poll();
    return () => {
      cancelled = true;
      requestController?.abort();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [enabled, fileIDsKey, intervalMs, loadStatuses]);
}

export function useFileProcessingStatusPolling(
  options: Omit<FileStatusPollingOptions<FileProcessingStatusDTO>, "loadStatuses" | "selectStatuses">,
) {
  useFileStatusPolling({
    ...options,
    loadStatuses: getFileProcessingStatuses,
    selectStatuses: selectStatusArray,
  });
}
