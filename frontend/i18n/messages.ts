import enAdminAnnouncements from "@/i18n/messages/en-US/admin-announcements.json";
import enAdminBilling from "@/i18n/messages/en-US/admin-billing.json";
import enAdminConversation from "@/i18n/messages/en-US/admin-conversation.json";
import enAdminFiles from "@/i18n/messages/en-US/admin-files.json";
import enAdminGroups from "@/i18n/messages/en-US/admin-groups.json";
import enAdminLogin from "@/i18n/messages/en-US/admin-login.json";
import enAdminLogs from "@/i18n/messages/en-US/admin-logs.json";
import enAdminModels from "@/i18n/messages/en-US/admin-models.json";
import enAdminPrompts from "@/i18n/messages/en-US/admin-prompts.json";
import enAdminTools from "@/i18n/messages/en-US/admin-tools.json";
import enAdminUpstreams from "@/i18n/messages/en-US/admin-upstreams.json";
import enAdminUsers from "@/i18n/messages/en-US/admin-users.json";
import enChat from "@/i18n/messages/en-US/chat.json";
import enAnnouncements from "@/i18n/messages/en-US/announcements.json";
import enCommon from "@/i18n/messages/en-US/common.json";
import enConversation from "@/i18n/messages/en-US/conversation.json";
import enErrors from "@/i18n/messages/en-US/errors.json";
import enFiles from "@/i18n/messages/en-US/files.json";
import enGuide from "@/i18n/messages/en-US/guide.json";
import enLogin from "@/i18n/messages/en-US/login.json";
import enPrompts from "@/i18n/messages/en-US/prompts.json";
import enRecent from "@/i18n/messages/en-US/recent.json";
import enSettings from "@/i18n/messages/en-US/settings.json";
import enShare from "@/i18n/messages/en-US/share.json";
import type { AppLocale } from "@/i18n/config";
import { replaceDefaultBrandTitle } from "@/shared/lib/branding";

const ENGLISH_MESSAGES = {
  common: enCommon,
  conversation: enConversation,
  errors: enErrors,
  login: enLogin,
  prompts: enPrompts,
  guide: enGuide,
  chat: enChat,
  announcements: enAnnouncements,
  recent: enRecent,
  share: enShare,
  files: enFiles,
  settings: enSettings,
  adminAnnouncements: enAdminAnnouncements,
  adminBilling: enAdminBilling,
  adminConversation: enAdminConversation,
  adminFiles: enAdminFiles,
  adminGroups: enAdminGroups,
  adminLogin: enAdminLogin,
  adminLogs: enAdminLogs,
  adminModels: enAdminModels,
  adminPrompts: enAdminPrompts,
  adminTools: enAdminTools,
  adminUpstreams: enAdminUpstreams,
  adminUsers: enAdminUsers,
};

export type AppMessages = typeof ENGLISH_MESSAGES;

function prepareMessages(messages: AppMessages): AppMessages {
  return {
    ...messages,
    guide: {
      ...messages.guide,
      userWelcomeTitle: replaceDefaultBrandTitle(messages.guide.userWelcomeTitle),
    },
    recent: {
      ...messages.recent,
      allConversationsDescription: replaceDefaultBrandTitle(messages.recent.allConversationsDescription),
    },
    login: {
      ...messages.login,
      title: replaceDefaultBrandTitle(messages.login.title),
    },
    share: {
      ...messages.share,
      signInToContinue: replaceDefaultBrandTitle(messages.share.signInToContinue),
    },
    chat: {
      ...messages.chat,
      placeholder: replaceDefaultBrandTitle(messages.chat.placeholder),
    },
    settings: {
      ...messages.settings,
      accountPage: {
        ...messages.settings.accountPage,
        securityDialog: {
          ...messages.settings.accountPage.securityDialog,
          email: {
            ...messages.settings.accountPage.securityDialog.email,
            description: {
              ...messages.settings.accountPage.securityDialog.email.description,
              change: replaceDefaultBrandTitle(
                messages.settings.accountPage.securityDialog.email.description.change,
              ),
            },
          },
        },
      },
    },
  };
}

export const DEFAULT_MESSAGES: AppMessages = prepareMessages(ENGLISH_MESSAGES);

export async function loadLocaleMessages(locale: AppLocale): Promise<AppMessages> {
  if (locale === "en-US") {
    return DEFAULT_MESSAGES;
  }

  const [
    common,
    conversation,
    errors,
    login,
    prompts,
    guide,
    chat,
    announcements,
    recent,
    share,
    files,
    settings,
    adminAnnouncements,
    adminBilling,
    adminConversation,
    adminFiles,
    adminGroups,
    adminLogin,
    adminLogs,
    adminModels,
    adminPrompts,
    adminTools,
    adminUpstreams,
    adminUsers,
  ] = await Promise.all([
    import("@/i18n/messages/zh-CN/common.json"),
    import("@/i18n/messages/zh-CN/conversation.json"),
    import("@/i18n/messages/zh-CN/errors.json"),
    import("@/i18n/messages/zh-CN/login.json"),
    import("@/i18n/messages/zh-CN/prompts.json"),
    import("@/i18n/messages/zh-CN/guide.json"),
    import("@/i18n/messages/zh-CN/chat.json"),
    import("@/i18n/messages/zh-CN/announcements.json"),
    import("@/i18n/messages/zh-CN/recent.json"),
    import("@/i18n/messages/zh-CN/share.json"),
    import("@/i18n/messages/zh-CN/files.json"),
    import("@/i18n/messages/zh-CN/settings.json"),
    import("@/i18n/messages/zh-CN/admin-announcements.json"),
    import("@/i18n/messages/zh-CN/admin-billing.json"),
    import("@/i18n/messages/zh-CN/admin-conversation.json"),
    import("@/i18n/messages/zh-CN/admin-files.json"),
    import("@/i18n/messages/zh-CN/admin-groups.json"),
    import("@/i18n/messages/zh-CN/admin-login.json"),
    import("@/i18n/messages/zh-CN/admin-logs.json"),
    import("@/i18n/messages/zh-CN/admin-models.json"),
    import("@/i18n/messages/zh-CN/admin-prompts.json"),
    import("@/i18n/messages/zh-CN/admin-tools.json"),
    import("@/i18n/messages/zh-CN/admin-upstreams.json"),
    import("@/i18n/messages/zh-CN/admin-users.json"),
  ]);

  return prepareMessages({
    common: common.default,
    conversation: conversation.default,
    errors: errors.default,
    login: login.default,
    prompts: prompts.default,
    guide: guide.default,
    chat: chat.default,
    announcements: announcements.default,
    recent: recent.default,
    share: share.default,
    files: files.default,
    settings: settings.default,
    adminAnnouncements: adminAnnouncements.default,
    adminBilling: adminBilling.default,
    adminConversation: adminConversation.default,
    adminFiles: adminFiles.default,
    adminGroups: adminGroups.default,
    adminLogin: adminLogin.default,
    adminLogs: adminLogs.default,
    adminModels: adminModels.default,
    adminPrompts: adminPrompts.default,
    adminTools: adminTools.default,
    adminUpstreams: adminUpstreams.default,
    adminUsers: adminUsers.default,
  });
}
