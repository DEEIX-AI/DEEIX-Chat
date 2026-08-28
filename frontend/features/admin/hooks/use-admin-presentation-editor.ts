import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import {
  createAdminLLMModelDisplayGroup,
  createAdminLLMModelVendor,
  deleteAdminLLMModelDisplayGroup,
  deleteAdminLLMModelVendor,
  listAdminLLMModels,
  updateAdminLLMModelDisplayGroup,
  updateAdminLLMModelVendor,
} from "@/features/admin/api";
import type {
  AdminLLMModelDisplayGroupDTO,
  AdminLLMModelDTO,
  AdminLLMModelVendorDeleteConflictDetails,
  AdminLLMModelVendorDTO,
} from "@/features/admin/api/llm.types";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { ApiError } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";

export type PresentationTab = "vendors" | "groups";

export type PresentationDeleteTarget =
  | { kind: "vendors"; key: string; name: string }
  | { kind: "groups"; id: number; name: string };

export type ModelPresentationEditorState = {
  kind: PresentationTab;
  key: string;
  id: number | null;
  creating: boolean;
  name: string;
  icon: string;
  modelIDs: number[];
  membersDirty: boolean;
};

const EMPTY_EDITOR: ModelPresentationEditorState = {
  kind: "vendors",
  key: "",
  id: null,
  creating: true,
  name: "",
  icon: "",
  modelIDs: [],
  membersDirty: false,
};

export function useAdminPresentationEditor({
  onChanged,
  onClose,
}: {
  onChanged: () => Promise<void>;
  onClose: () => void;
}) {
  const t = useTranslations("adminModels.presentation");
  const [editor, setEditor] = React.useState<ModelPresentationEditorState | null>(null);
  const stableEditor = useDialogSnapshot(editor);
  const [pending, setPending] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<PresentationDeleteTarget | null>(null);
  const [catalogModels, setCatalogModels] = React.useState<AdminLLMModelDTO[] | null>(null);
  const [modelsLoading, setModelsLoading] = React.useState(false);
  const [modelQuery, setModelQuery] = React.useState("");

  const loadCatalogModels = React.useCallback(async () => {
    setModelsLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMModels(token, { ...options, onlyActive: false, sort: "sortOrder_asc" }),
      );
      setCatalogModels(results);
    } catch (error) {
      setCatalogModels(null);
      toast.error(t("modelsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setModelsLoading(false);
    }
  }, [t]);

  const closeDialog = React.useCallback(() => {
    if (pending) {
      return;
    }
    setEditor(null);
    onClose();
  }, [onClose, pending]);

  const openCreate = React.useCallback((kind: PresentationTab) => {
    if (kind === "groups" && catalogModels === null) {
      return;
    }
    setModelQuery("");
    setEditor({ ...EMPTY_EDITOR, kind });
  }, [catalogModels]);

  const openVendorEdit = React.useCallback((vendor: AdminLLMModelVendorDTO) => {
    setEditor({
      kind: "vendors",
      key: vendor.key,
      id: null,
      creating: false,
      name: vendor.name,
      icon: vendor.icon,
      modelIDs: [],
      membersDirty: false,
    });
  }, []);

  const openGroupEdit = React.useCallback((group: AdminLLMModelDisplayGroupDTO) => {
    if (catalogModels === null) {
      return;
    }
    setModelQuery("");
    setEditor({
      kind: "groups",
      key: "",
      id: group.id,
      creating: false,
      name: group.name,
      icon: group.icon,
      modelIDs: catalogModels.filter((model) => model.displayGroupID === group.id).map((model) => model.id),
      membersDirty: false,
    });
  }, [catalogModels]);

  const toggleEditorModel = React.useCallback((modelID: number, checked: boolean) => {
    setEditor((current) => {
      if (!current || current.kind !== "groups") {
        return current;
      }
      const selected = new Set(current.modelIDs);
      if (checked) {
        selected.add(modelID);
      } else {
        selected.delete(modelID);
      }
      return { ...current, modelIDs: Array.from(selected), membersDirty: true };
    });
  }, []);

  const saveEditor = React.useCallback(async () => {
    if (!editor || pending) {
      return;
    }
    const name = editor.name.trim();
    const key = editor.key.trim().toLowerCase();
    if (!name || (editor.kind === "vendors" && !key)) {
      toast.error(t("validationRequired"));
      return;
    }

    setPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      const payload = { name, icon: editor.icon.trim() };
      if (editor.kind === "vendors") {
        if (editor.creating) {
          await createAdminLLMModelVendor(token, { key, ...payload });
        } else {
          await updateAdminLLMModelVendor(token, key, payload);
        }
      } else {
        const groupPayload = editor.creating || editor.membersDirty
          ? { ...payload, modelIDs: editor.modelIDs }
          : payload;
        await (editor.id
          ? updateAdminLLMModelDisplayGroup(token, editor.id, groupPayload)
          : createAdminLLMModelDisplayGroup(token, groupPayload));
      }
      await onChanged();
      if (editor.kind === "groups") {
        await loadCatalogModels();
      }
      setEditor(null);
      toast.success(t("saved"));
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [editor, loadCatalogModels, onChanged, pending, t]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget || pending) {
      return;
    }
    setPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      if (deleteTarget.kind === "vendors") {
        await deleteAdminLLMModelVendor(token, deleteTarget.key);
      } else {
        await deleteAdminLLMModelDisplayGroup(token, deleteTarget.id);
      }
      await onChanged();
      if (deleteTarget.kind === "groups") {
        await loadCatalogModels();
      }
      setDeleteTarget(null);
      setEditor((current) => {
        if (!current || current.kind !== deleteTarget.kind) {
          return current;
        }
        if (deleteTarget.kind === "vendors") {
          return current.key === deleteTarget.key ? null : current;
        }
        return current.id === deleteTarget.id ? null : current;
      });
      toast.success(t(deleteTarget.kind === "vendors" ? "vendorDeleted" : "groupDeleted"));
    } catch (error) {
      if (
        deleteTarget.kind === "vendors" &&
        error instanceof ApiError &&
        error.errorCode === "llm.model_vendor_in_use" &&
        isModelVendorDeleteConflictDetails(error.details)
      ) {
        const names = error.details.models
          .map((model) => typeof model.platformModelName === "string" ? model.platformModelName.trim() : "")
          .filter(Boolean);
        const hiddenCount = Math.max(0, error.details.referenceCount - names.length);
        setDeleteTarget(null);
        toast.error(t("vendorInUseTitle"), {
          description: t("vendorInUseDescription", {
            count: error.details.referenceCount,
            models: names.join(", "),
            more: hiddenCount > 0 ? t("vendorInUseMore", { count: hiddenCount }) : "",
          }),
        });
      } else {
        toast.error(t("deleteFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      setPending(false);
    }
  }, [deleteTarget, loadCatalogModels, onChanged, pending, t]);

  return {
    editor,
    setEditor,
    stableEditor,
    pending,
    deleteTarget,
    setDeleteTarget,
    catalogModels,
    modelsLoading,
    modelQuery,
    setModelQuery,
    loadCatalogModels,
    closeDialog,
    openCreate,
    openVendorEdit,
    openGroupEdit,
    toggleEditorModel,
    saveEditor,
    confirmDelete,
  };
}

function isModelVendorDeleteConflictDetails(value: unknown): value is AdminLLMModelVendorDeleteConflictDetails {
  if (!value || typeof value !== "object") {
    return false;
  }
  const details = value as Partial<AdminLLMModelVendorDeleteConflictDetails>;
  return typeof details.referenceCount === "number" && Array.isArray(details.models);
}
