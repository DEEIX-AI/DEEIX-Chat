"use client";

import * as React from "react";

import { subscribeFileLibraryInvalidated, type FileLibraryInvalidatedDetail } from "@/entities/file";

export function useFileInvalidation(
  onInvalidated: (detail: FileLibraryInvalidatedDetail) => void,
) {
  const handlerRef = React.useRef(onInvalidated);

  React.useEffect(() => {
    handlerRef.current = onInvalidated;
  }, [onInvalidated]);

  React.useEffect(() => {
    return subscribeFileLibraryInvalidated((detail) => {
      handlerRef.current(detail);
    });
  }, []);
}
