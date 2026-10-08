// Public entry of the admin feature: the /admin layout mounts the gate and shell,
// and each /admin/<section> route mounts its section entry. Unused re-exports are
// dropped at build time because package.json declares `sideEffects`, so a section
// route does not bundle the other sections.
export { AdminAccessGate } from "@/features/admin/components/admin-access-gate";
export { AdminShell } from "@/features/admin/components/admin-shell";
export { AdminAboutPage } from "@/features/admin/components/sections/about/admin-about";
export { AdminAnnouncementsPage } from "@/features/admin/components/sections/announcements/admin-announcements";
export { AdminBillingPage } from "@/features/admin/components/sections/billing/admin-billing";
export { AdminContentModerationPage } from "@/features/admin/components/sections/content-moderation/admin-content-moderation";
export { AdminConversationPage } from "@/features/admin/components/sections/conversation/admin-conversation";
export { AdminFilesPage } from "@/features/admin/components/sections/files/admin-files";
export { AdminGroupsPage } from "@/features/admin/components/sections/groups/admin-groups";
export { AdminKnowledgeBasesPage } from "@/features/admin/components/sections/knowledge-bases/admin-knowledge-bases";
export { AdminLoginPage } from "@/features/admin/components/sections/login/admin-login";
export { AdminLogsPage } from "@/features/admin/components/sections/logs/admin-logs";
export { AdminModelsPage } from "@/features/admin/components/sections/models/admin-models";
export { AdminStatisticsPage } from "@/features/admin/components/sections/statistics/admin-statistics";
export { AdminToolsPage } from "@/features/admin/components/sections/tools/admin-tools";
export { AdminUpstreamsPage } from "@/features/admin/components/sections/upstreams/admin-upstreams";
export { AdminUserKeysPage } from "@/features/admin/components/sections/user-keys/admin-user-keys";
export { AdminUsersPage } from "@/features/admin/components/sections/users/admin-users";
