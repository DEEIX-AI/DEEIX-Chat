import * as React from "react"

let mobileQuery: MediaQueryList | null = null
const subscribers = new Set<() => void>()

function getMobileQuery() {
  if (typeof window === "undefined") {
    return null
  }
  mobileQuery ??= window.matchMedia("(max-width: 767px)")
  return mobileQuery
}

function getMobileSnapshot() {
  return getMobileQuery()?.matches ?? false
}

function subscribeToMobileQuery(subscriber: () => void) {
  const query = getMobileQuery()
  subscribers.add(subscriber)
  if (subscribers.size === 1) {
    query?.addEventListener("change", notifyMobileSubscribers)
  }

  return () => {
    subscribers.delete(subscriber)
    if (subscribers.size === 0) {
      query?.removeEventListener("change", notifyMobileSubscribers)
    }
  }
}

function notifyMobileSubscribers() {
  for (const subscriber of subscribers) {
    subscriber()
  }
}

export function useIsMobile() {
  return React.useSyncExternalStore(
    subscribeToMobileQuery,
    getMobileSnapshot,
    () => false,
  )
}
