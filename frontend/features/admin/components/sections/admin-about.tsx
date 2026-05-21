"use client";

import { useTranslations } from "next-intl";

import { AboutSettingsContent } from "@/shared/components/about-settings-content";

export function AdminAboutPage() {
  const t = useTranslations("adminUsers.aboutPage");

  return (
    <AboutSettingsContent
      title={t("title")}
      description={t("description")}
      consoleLabel={t("adminConsole")}
      labels={{
        details: t("details"),
        official: t("official"),
        website: t("website"),
        repository: t("repository"),
        social: t("social"),
        blog: t("blog"),
        contact: t("contact"),
        copyright: t("copyright"),
        license: t("license"),
      }}
    />
  );
}
