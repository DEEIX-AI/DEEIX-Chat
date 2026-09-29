export type ModelPresentationSource = {
  vendor?: string | null;
  vendorName?: string | null;
  vendorIcon?: string | null;
  displayGroupID?: number | null;
  displayGroupName?: string | null;
  displayGroupIcon?: string | null;
};

export type ResolvedModelPresentationGroup = {
  key: string;
  label: string;
  icon: string;
};

// resolveModelPresentationGroup returns the optional display group when set;
// otherwise the model keeps its technical-vendor grouping.
export function resolveModelPresentationGroup(
  model: ModelPresentationSource,
): ResolvedModelPresentationGroup {
  const displayGroupID = model.displayGroupID ?? 0;
  const displayGroupName = model.displayGroupName?.trim() ?? "";
  if (displayGroupID > 0 && displayGroupName) {
    return {
      key: `group:${displayGroupID}`,
      label: displayGroupName,
      icon: model.displayGroupIcon?.trim() ?? "",
    };
  }

  const vendorKey = model.vendor?.trim().toLowerCase() || "unknown";
  return {
    key: `vendor:${vendorKey}`,
    label: model.vendorName?.trim() || model.vendor?.trim() || vendorKey,
    icon: model.vendorIcon?.trim() ?? "",
  };
}
