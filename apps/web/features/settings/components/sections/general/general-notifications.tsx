"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Switch } from "@/components/ui/switch";
import { SettingsFieldRow, SettingsSection } from "@/shared/components/settings-layout";

export function GeneralNotificationsSection({
  notificationRuntimeReady,
  notificationSupported,
  responseCompletionNotificationsEnabled,
  notificationHelpText,
  onResponseCompletionNotificationsChange,
}: {
  notificationRuntimeReady: boolean;
  notificationSupported: boolean;
  responseCompletionNotificationsEnabled: boolean;
  notificationHelpText: React.ReactNode;
  onResponseCompletionNotificationsChange: (checked: boolean) => void;
}) {
  const t = useTranslations("settings");

  return (
    <SettingsSection title={t("notifications")}>
      <SettingsFieldRow
        title={t("generalPage.notifications.responseCompletionTitle")}
        description={
          <>
            {t("generalPage.notifications.responseCompletionDescription")}
            <br />
            {notificationHelpText}
          </>
        }
        controlClassName="sm:w-auto md:w-auto"
      >
        <Switch
          checked={responseCompletionNotificationsEnabled}
          onCheckedChange={onResponseCompletionNotificationsChange}
          aria-label={t("generalPage.notifications.toggleResponseCompletion")}
          disabled={!notificationRuntimeReady || !notificationSupported}
          className="shrink-0"
        />
      </SettingsFieldRow>
    </SettingsSection>
  );
}
