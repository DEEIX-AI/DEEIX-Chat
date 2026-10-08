import type { Feature } from "@deeix/core";

// Which server capability a section depends on, if any. The sidebar and the
// route guard both read this table, so a hidden entry is never reachable by URL
// either (docs/ARCHITECTURE.md §4). Each id equals its route segment and its
// sections/<id>/ component directory.
export const ADMIN_SECTIONS = [
  { id: "statistics", label: "Statistics", href: "/statistics" },
  { id: "users", label: "Accounts", href: "/users", feature: "multiUser" },
  { id: "groups", label: "Permission Groups", href: "/groups", feature: "multiUser" },
  { id: "upstreams", label: "Upstreams", href: "/upstreams" },
  { id: "models", label: "Models", href: "/models" },
  { id: "tools", label: "Tools", href: "/tools" },
  { id: "billing", label: "Billing", href: "/billing" },
  { id: "announcements", label: "Announcements", href: "/announcements", feature: "announcements" },
  { id: "logs", label: "Logs", href: "/logs" },
  { id: "content-moderation", label: "Content moderation", href: "/content-moderation", feature: "contentModeration" },
  { id: "login", label: "Login & auth", href: "/login", feature: "identityProviders" },
  { id: "conversation", label: "Conversation", href: "/conversation" },
  { id: "files", label: "Files & retrieval", href: "/files" },
  { id: "knowledge-bases", label: "Knowledge bases", href: "/knowledge-bases" },
  // Users' own API keys: there is no other account to own one in single-user (local) mode.
  { id: "user-keys", label: "User model providers", href: "/user-keys", feature: "multiUser" },
  { id: "about", label: "About", href: "/about" },
] as const satisfies readonly { id: string; label: string; href: string; feature?: Feature }[];

export type AdminSection = (typeof ADMIN_SECTIONS)[number]["id"];
type AdminSectionEntry = (typeof ADMIN_SECTIONS)[number];

export const DEFAULT_ADMIN_SECTION: AdminSection = "statistics";

/** The section that owns `pathname` under `basePath`, or null for none. */
export function resolveAdminSectionFromPath(pathname: string, basePath: string): AdminSectionEntry | null {
  const normalizedBasePath = basePath.replace(/\/$/, "");
  return (
    ADMIN_SECTIONS.find((entry) => {
      const href = `${normalizedBasePath}${entry.href}`;
      return pathname === href || pathname.startsWith(`${href}/`);
    }) ?? null
  );
}
