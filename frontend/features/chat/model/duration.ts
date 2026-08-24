type DurationMS = number | null | undefined;

function validDurationMS(value: DurationMS): number | undefined {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;
}

export function durationBetweenMS(startedAt: string | undefined, endedAt: string | undefined): number | undefined {
  if (!startedAt || !endedAt) {
    return undefined;
  }

  const startedMS = new Date(startedAt).getTime();
  const endedMS = new Date(endedAt).getTime();
  if (!Number.isFinite(startedMS) || !Number.isFinite(endedMS)) {
    return undefined;
  }
  return validDurationMS(endedMS - startedMS);
}

export function firstDurationMS(...values: DurationMS[]): number | undefined {
  for (const value of values) {
    const durationMS = validDurationMS(value);
    if (durationMS !== undefined) {
      return durationMS;
    }
  }
  return undefined;
}

export function sumDurationsMS(values: Iterable<DurationMS>): number | undefined {
  let totalMS = 0;
  for (const value of values) {
    totalMS += validDurationMS(value) ?? 0;
  }
  return validDurationMS(totalMS);
}

export function formatDurationMS(value: DurationMS): string | undefined {
  const durationMS = validDurationMS(value);
  if (durationMS === undefined) {
    return undefined;
  }

  const seconds = durationMS / 1000;
  if (seconds < 10) {
    return `${Math.max(0.1, seconds).toFixed(1)}s`;
  }
  return `${Math.round(seconds)}s`;
}
