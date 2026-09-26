export type RecentDeleteTarget = { ids: string[]; label: string } | null;
export type RecentBulkConfirmAction = "archive" | "unarchive" | "revokeShares";

export type RecentRowState = {
  publicID: string;
  hovered: boolean;
  selected: boolean;
  highlighted: boolean;
};
