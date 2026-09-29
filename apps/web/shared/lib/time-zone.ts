export function detectCurrentTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "Etc/UTC";
  } catch {
    return "Etc/UTC";
  }
}

export function resolveTimeZoneOptions(): string[] {
  const options = new Set<string>(["Etc/UTC"]);
  // lib.es2022 declares supportedValuesOf unconditionally; older WebKit builds lack it.
  const timeZones = typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : [];
  for (const timeZone of timeZones) {
    options.add(timeZone);
  }

  return Array.from(options).sort((left, right) => left.localeCompare(right));
}
