"use client"

import * as React from "react"

export function useStoredBoolean(storageKey: string, defaultValue: boolean) {
  const [value, setValue] = React.useState(() => {
    if (typeof window === "undefined") {
      return defaultValue
    }

    try {
      const stored = window.localStorage.getItem(storageKey)
      if (stored === "true") {
        return true
      }
      if (stored === "false") {
        return false
      }
    } catch {
      return defaultValue
    }

    return defaultValue
  })

  React.useEffect(() => {
    try {
      window.localStorage.setItem(storageKey, value ? "true" : "false")
    } catch {
      // localStorage can be unavailable in private browsing or strict environments.
    }
  }, [storageKey, value])

  return [value, setValue] as const
}
