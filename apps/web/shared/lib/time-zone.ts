export function detectCurrentTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "Etc/UTC";
  } catch {
    return "Etc/UTC";
  }
}

export function resolveTimeZoneOptions(): string[] {
  const options = new Set<string>(["Etc/UTC"]);
  const intlWithSupportedValues = Intl as typeof Intl & {
    supportedValuesOf?: (key: "timeZone") => string[];
  };

  for (const timeZone of intlWithSupportedValues.supportedValuesOf?.("timeZone") ?? []) {
    options.add(timeZone);
  }

  return Array.from(options).sort((left, right) => left.localeCompare(right));
}
