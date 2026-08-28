"use client";

import * as React from "react";

const SUBSECOND_TICK_MS = 100;
const WHOLE_SECOND_TICK_MS = 1000;

export function useChatElapsedDurationMS(active: boolean, startedAt: string | undefined): number | undefined {
  const [elapsedMS, setElapsedMS] = React.useState<number>();

  React.useEffect(() => {
    if (!active || !startedAt) {
      setElapsedMS(undefined);
      return;
    }

    const startedMS = new Date(startedAt).getTime();
    if (!Number.isFinite(startedMS)) {
      setElapsedMS(undefined);
      return;
    }

    let timerID: number | undefined;
    const tick = () => {
      const nextElapsedMS = Math.max(0, Date.now() - startedMS);
      setElapsedMS(nextElapsedMS);

      const intervalMS = nextElapsedMS < 10_000 ? SUBSECOND_TICK_MS : WHOLE_SECOND_TICK_MS;
      const delayMS = Math.max(1, intervalMS - (nextElapsedMS % intervalMS));
      timerID = window.setTimeout(tick, delayMS);
    };

    tick();
    return () => {
      if (timerID !== undefined) {
        window.clearTimeout(timerID);
      }
    };
  }, [active, startedAt]);

  return active ? elapsedMS : undefined;
}
