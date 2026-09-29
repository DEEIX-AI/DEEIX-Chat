import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { orderSkills, type SkillListItem, skillKey } from "@/features/library/model/library-skill-list";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { createMySkill, deleteMySkill, getVisibleSkill, listMySkills, listVisibleSkills, updateMySkill } from "@/shared/api/skills";
import type { SkillDTO } from "@/shared/api/skills-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { skillFormIsWithinLimits, skillPayloadFromForm, skillPayloadIsComplete, type SkillFormValue } from "@/entities/skill";

const SKILL_PAGE_SIZE = 100;

/**
 * The user's own skills plus the built-in ones, with create/update/delete and
 * an optimistic enabled toggle that rolls back on failure.
 */
export function useLibrarySkillList() {
  const t = useTranslations("library");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<SkillListItem[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  const reload = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setItems([]);
        return;
      }
      const [mine, visible] = await Promise.all([
        listMySkills(token, { page: 1, pageSize: SKILL_PAGE_SIZE }),
        listVisibleSkills(token, { page: 1, pageSize: SKILL_PAGE_SIZE }),
      ]);
      setItems(orderSkills([...mine.results, ...visible.results.filter((item) => item.scope === "builtin")]));
    } catch (error) {
      toast.error(t("skillsLoadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [resolveErrorMessage, t]);

  React.useEffect(() => {
    void reload();
  }, [reload]);

  /** Loads the full markdown of a read-only (built-in) skill; resolves null when unavailable. */
  const loadVisibleSkill = React.useCallback(async (id: number): Promise<SkillDTO | null> => {
    try {
      const token = await resolveAccessToken();
      if (!token) return null;
      const data = await getVisibleSkill(token, id);
      return data.skill;
    } catch (error) {
      toast.error(t("skillsLoadFailed"), { description: resolveErrorMessage(error) });
      return null;
    }
  }, [resolveErrorMessage, t]);

  // `onSaved` runs inside the request flow (before `saving` resets) so the
  // dialog closes in the same update as before the extraction.
  const saveSkill = React.useCallback(async (form: SkillFormValue, onSaved: () => void) => {
    const payload = skillPayloadFromForm(form);
    if (!skillPayloadIsComplete(payload)) {
      toast.error(t("skillInvalid"));
      return;
    }
    if (!skillFormIsWithinLimits(form)) {
      toast.error(t("skillTooLong"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      if (form.id) {
        const data = await updateMySkill(token, form.id, payload);
        setItems((current) => orderSkills(current.map((item) => (skillKey(item) === skillKey(data.skill) ? data.skill : item))));
        toast.success(t("skillUpdated"));
      } else {
        const data = await createMySkill(token, payload);
        setItems((current) => orderSkills([...current, data.skill]));
        toast.success(t("skillCreated"));
      }
      onSaved();
    } catch (error) {
      toast.error(form.id ? t("skillUpdateFailed") : t("skillCreateFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [resolveErrorMessage, t]);

  const deleteSkill = React.useCallback(async (target: SkillDTO) => {
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await deleteMySkill(token, target.id);
      setItems((current) => current.filter((item) => skillKey(item) !== skillKey(target)));
      toast.success(t("skillDeleted"));
    } catch (error) {
      toast.error(t("skillDeleteFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  const toggleEnabled = React.useCallback(
    async (item: SkillDTO, enabled: boolean) => {
      if (item.scope !== "user") return;
      const previous = item;
      setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(item) ? { ...row, enabled } : row))));
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const data = await updateMySkill(token, item.id, { enabled });
        setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(data.skill) ? data.skill : row))));
      } catch (error) {
        setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(previous) ? previous : row))));
        toast.error(t("skillUpdateFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [resolveErrorMessage, t],
  );

  return { items, loading, saving, loadVisibleSkill, saveSkill, deleteSkill, toggleEnabled };
}
