import * as React from "react";

import type { UserDTO } from "@/shared/api/auth.types";

type UseAdminUserSelectionState = {
  selectedUserIDs: Set<number>;
  selectAllState: boolean | "indeterminate";
  resolveSelectedUsers: () => UserDTO[];
  handleSelectAllVisible: (checked: boolean) => void;
  handleToggleSelectedUser: (userID: number, checked: boolean) => void;
  setSelectedUserIDs: React.Dispatch<React.SetStateAction<Set<number>>>;
};

export function useAdminUserSelection(items: UserDTO[], filteredItems: UserDTO[]): UseAdminUserSelectionState {
  const [selectedUserIDs, setSelectedUserIDs] = React.useState<Set<number>>(new Set());

  React.useEffect(() => {
    const itemIDs = new Set(items.map((item) => item.id));
    setSelectedUserIDs((current) => {
      const next = new Set<number>();
      current.forEach((userID) => {
        if (itemIDs.has(userID)) {
          next.add(userID);
        }
      });
      return next.size === current.size ? current : next;
    });
  }, [items]);

  React.useEffect(() => {
    const visibleIDs = new Set(filteredItems.map((item) => item.id));
    setSelectedUserIDs((current) => {
      const next = new Set<number>();
      current.forEach((userID) => {
        if (visibleIDs.has(userID)) {
          next.add(userID);
        }
      });
      return next.size === current.size ? current : next;
    });
  }, [filteredItems]);

  const visibleSelectedCount = React.useMemo(
    () => filteredItems.filter((item) => selectedUserIDs.has(item.id)).length,
    [filteredItems, selectedUserIDs],
  );

  const selectAllState: boolean | "indeterminate" =
    filteredItems.length === 0
      ? false
      : visibleSelectedCount === filteredItems.length
        ? true
        : visibleSelectedCount > 0
          ? "indeterminate"
          : false;

  const handleSelectAllVisible = React.useCallback(
    (checked: boolean) => {
      const visibleIDs = filteredItems.map((item) => item.id);
      setSelectedUserIDs((current) => {
        const currentSet = new Set(current);
        if (checked) {
          for (const id of visibleIDs) {
            currentSet.add(id);
          }
        } else {
          for (const id of visibleIDs) {
            currentSet.delete(id);
          }
        }
        return currentSet;
      });
    },
    [filteredItems],
  );

  const handleToggleSelectedUser = React.useCallback((userID: number, checked: boolean) => {
    setSelectedUserIDs((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(userID);
      } else {
        next.delete(userID);
      }
      return next.size === current.size && next.has(userID) === current.has(userID) ? current : next;
    });
  }, []);

  const resolveSelectedUsers = React.useCallback(
    () => filteredItems.filter((item) => selectedUserIDs.has(item.id)),
    [filteredItems, selectedUserIDs],
  );

  return {
    selectedUserIDs,
    selectAllState,
    resolveSelectedUsers,
    handleSelectAllVisible,
    handleToggleSelectedUser,
    setSelectedUserIDs,
  };
}
