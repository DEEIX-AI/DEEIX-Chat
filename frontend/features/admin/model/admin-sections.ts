export const ADMIN_SECTIONS = [
  { id: "accounts", label: "Accounts", href: "/users" },
  { id: "upstreams", label: "Upstreams", href: "/upstreams" },
  { id: "models", label: "Models", href: "/models" },
  { id: "groups", label: "Groups", href: "/groups" },
  { id: "tool-settings", label: "Tools", href: "/tools" },
  { id: "billing", label: "Billing", href: "/billing" },
  { id: "announcements", label: "Announcements", href: "/announcements" },
  { id: "logs", label: "Logs", href: "/logs" },
  { id: "login-settings", label: "Login & auth", href: "/login" },
  { id: "conversation-settings", label: "Conversation", href: "/conversation" },
  { id: "chat-files", label: "Files & retrieval", href: "/chat-files" },
  { id: "about", label: "About", href: "/about" },
] as const;

export type AdminSection = (typeof ADMIN_SECTIONS)[number]["id"];

export function resolveAdminSection(section?: string | null): AdminSection {
  if (ADMIN_SECTIONS.some((item) => item.id === section)) {
    return section as AdminSection;
  }
  return "accounts";
}
