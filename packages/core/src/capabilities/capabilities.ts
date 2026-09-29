// Server capabilities: what a server offers, as declared by GET /api/v1/capabilities.
// Contract: docs/ARCHITECTURE.md §4. Clients render from these flags and never
// infer them from the platform they run on.

export const FEATURE_NAMES = [
  "multiUser",
  "registration",
  "identityProviders",
  "accountSecurity",
  "announcements",
  "billingGating",
  "usageMetering",
  "contentModeration",
  "sharing",
] as const;

export type Feature = (typeof FEATURE_NAMES)[number];

export type CapabilityFlags = Readonly<Record<Feature, boolean>>;

/**
 * Every feature enabled. This is the behaviour of a server that predates the
 * capabilities endpoint, so it is also the fallback when the endpoint cannot
 * be reached: a client must never hide a feature the server actually has.
 */
export const ALL_ENABLED: CapabilityFlags = Object.freeze(
  Object.fromEntries(FEATURE_NAMES.map((name) => [name, true])) as Record<Feature, boolean>,
);

/**
 * Turn a capabilities payload into a complete flag set. Tolerant by design:
 * unknown keys are ignored (a newer server may add features), missing or
 * non-boolean keys default to enabled (an older server does not know them).
 */
export function resolveCapabilities(input: unknown): CapabilityFlags {
  const features = featuresOf(input);
  const flags = { ...ALL_ENABLED } as Record<Feature, boolean>;
  for (const name of FEATURE_NAMES) {
    const value = features[name];
    if (typeof value === "boolean") {
      flags[name] = value;
    }
  }
  return Object.freeze(flags);
}

function featuresOf(input: unknown): Record<string, unknown> {
  if (!isRecord(input)) {
    return {};
  }
  // Accept both the bare `{ features }` object and the full envelope `{ data: { features } }`.
  const container = isRecord(input.data) ? input.data : input;
  return isRecord(container.features) ? container.features : {};
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Error code a server returns for an endpoint whose feature is disabled. */
export const FEATURE_DISABLED_ERROR_CODE = "feature.disabled";

/** Read the feature named by a `feature.disabled` error, if the payload carries one. */
export function disabledFeatureOf(error: unknown): Feature | null {
  if (!isRecord(error) || error.errorCode !== FEATURE_DISABLED_ERROR_CODE) {
    return null;
  }
  const details = error.details;
  if (!isRecord(details) || typeof details.feature !== "string") {
    return null;
  }
  return (FEATURE_NAMES as readonly string[]).includes(details.feature) ? (details.feature as Feature) : null;
}
