"use client";

import * as React from "react";

export function useDialogSnapshot<T>(value: T | null | undefined): T | null {
  const [snapshot, setSnapshot] = React.useState<T | null>(value ?? null);

  React.useEffect(() => {
    if (value !== null && value !== undefined) {
      setSnapshot(value);
    }
  }, [value]);

  return value ?? snapshot;
}
