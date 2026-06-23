import * as React from "react";

import type { UserDTO } from "@/shared/api/auth.types";
import type { UserSortValue } from "@/features/admin/types/accounts";

type UseAdminUserFiltersState = {
  roleFilter: string;
  setRoleFilter: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  tierFilter: string;
  setTierFilter: (value: string) => void;
  sortValue: UserSortValue;
  setSortValue: (value: UserSortValue) => void;
  filteredItems: UserDTO[];
};

export function useAdminUserFilters(items: UserDTO[]): UseAdminUserFiltersState {
  const [roleFilter, setRoleFilter] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [tierFilter, setTierFilter] = React.useState("");
  const [sortValue, setSortValue] = React.useState<UserSortValue>("id_desc");
  const deferredRoleFilter = React.useDeferredValue(roleFilter);
  const deferredStatusFilter = React.useDeferredValue(statusFilter);
  const deferredTierFilter = React.useDeferredValue(tierFilter);
  const deferredSortValue = React.useDeferredValue(sortValue);

  const filteredItems = React.useMemo(() => {
    const next = items.filter((item) => {
      const matchesRole = !deferredRoleFilter || item.role === deferredRoleFilter;
      const matchesStatus = !deferredStatusFilter || item.status === deferredStatusFilter;
      const matchesTier = !deferredTierFilter || item.subscriptionTier === deferredTierFilter;
      return matchesRole && matchesStatus && matchesTier;
    });

    const lastActiveTimestamps = new Map(next.map((item) => [item.id, new Date(item.lastActiveAt || item.lastLoginAt || 0).getTime()]));
    const updatedTimestamps = new Map(next.map((item) => [item.id, new Date(item.updatedAt || 0).getTime()]));

    next.sort((left, right) => {
      switch (deferredSortValue) {
        case "id_asc":
          return left.id - right.id;
        case "last_login_desc":
          return (lastActiveTimestamps.get(right.id) ?? 0) - (lastActiveTimestamps.get(left.id) ?? 0);
        case "updated_desc":
          return (updatedTimestamps.get(right.id) ?? 0) - (updatedTimestamps.get(left.id) ?? 0);
        case "display_name_asc":
          return (left.displayName || left.username).localeCompare(right.displayName || right.username, "zh-CN");
        case "id_desc":
        default:
          return right.id - left.id;
      }
    });

    return next;
  }, [deferredRoleFilter, deferredSortValue, deferredStatusFilter, deferredTierFilter, items]);

  return {
    roleFilter,
    setRoleFilter,
    statusFilter,
    setStatusFilter,
    tierFilter,
    setTierFilter,
    sortValue,
    setSortValue,
    filteredItems,
  };
}
