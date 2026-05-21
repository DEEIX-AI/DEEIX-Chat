import enAdminBilling from "@/i18n/messages/en-US/admin-billing.json";
import enAdminChannels from "@/i18n/messages/en-US/admin-channels.json";
import enAdminConversation from "@/i18n/messages/en-US/admin-conversation.json";
import enAdminFiles from "@/i18n/messages/en-US/admin-files.json";
import enAdminLogin from "@/i18n/messages/en-US/admin-login.json";
import enAdminLogs from "@/i18n/messages/en-US/admin-logs.json";
import enAdminModels from "@/i18n/messages/en-US/admin-models.json";
import enAdminTools from "@/i18n/messages/en-US/admin-tools.json";
import enAdminUsers from "@/i18n/messages/en-US/admin-users.json";
import enChat from "@/i18n/messages/en-US/chat.json";
import enCommon from "@/i18n/messages/en-US/common.json";
import enErrors from "@/i18n/messages/en-US/errors.json";
import enFiles from "@/i18n/messages/en-US/files.json";
import enGuide from "@/i18n/messages/en-US/guide.json";
import enLogin from "@/i18n/messages/en-US/login.json";
import enRecent from "@/i18n/messages/en-US/recent.json";
import enSettings from "@/i18n/messages/en-US/settings.json";
import enShare from "@/i18n/messages/en-US/share.json";
import type { AppLocale } from "@/i18n/config";

export type AppMessages = typeof DEFAULT_MESSAGES;

export const DEFAULT_MESSAGES = {
  common: enCommon,
  errors: enErrors,
  login: enLogin,
  guide: enGuide,
  chat: enChat,
  recent: enRecent,
  share: enShare,
  files: enFiles,
  settings: enSettings,
  adminUsers: enAdminUsers,
  adminChannels: enAdminChannels,
  adminConversation: enAdminConversation,
  adminFiles: enAdminFiles,
  adminLogin: enAdminLogin,
  adminModels: enAdminModels,
  adminBilling: enAdminBilling,
  adminLogs: enAdminLogs,
  adminTools: enAdminTools,
};

export async function loadLocaleMessages(locale: AppLocale): Promise<AppMessages> {
  if (locale === "en-US") {
    return DEFAULT_MESSAGES;
  }

  const [
    common,
    errors,
    login,
    guide,
    chat,
    recent,
    share,
    files,
    settings,
    adminUsers,
    adminChannels,
    adminConversation,
    adminFiles,
    adminLogin,
    adminModels,
    adminBilling,
    adminLogs,
    adminTools,
  ] = await Promise.all([
    import("@/i18n/messages/zh-CN/common.json"),
    import("@/i18n/messages/zh-CN/errors.json"),
    import("@/i18n/messages/zh-CN/login.json"),
    import("@/i18n/messages/zh-CN/guide.json"),
    import("@/i18n/messages/zh-CN/chat.json"),
    import("@/i18n/messages/zh-CN/recent.json"),
    import("@/i18n/messages/zh-CN/share.json"),
    import("@/i18n/messages/zh-CN/files.json"),
    import("@/i18n/messages/zh-CN/settings.json"),
    import("@/i18n/messages/zh-CN/admin-users.json"),
    import("@/i18n/messages/zh-CN/admin-channels.json"),
    import("@/i18n/messages/zh-CN/admin-conversation.json"),
    import("@/i18n/messages/zh-CN/admin-files.json"),
    import("@/i18n/messages/zh-CN/admin-login.json"),
    import("@/i18n/messages/zh-CN/admin-models.json"),
    import("@/i18n/messages/zh-CN/admin-billing.json"),
    import("@/i18n/messages/zh-CN/admin-logs.json"),
    import("@/i18n/messages/zh-CN/admin-tools.json"),
  ]);

  return {
    common: common.default,
    errors: errors.default,
    login: login.default,
    guide: guide.default,
    chat: chat.default,
    recent: recent.default,
    share: share.default,
    files: files.default,
    settings: settings.default,
    adminUsers: adminUsers.default,
    adminChannels: adminChannels.default,
    adminConversation: adminConversation.default,
    adminFiles: adminFiles.default,
    adminLogin: adminLogin.default,
    adminModels: adminModels.default,
    adminBilling: adminBilling.default,
    adminLogs: adminLogs.default,
    adminTools: adminTools.default,
  };
}
