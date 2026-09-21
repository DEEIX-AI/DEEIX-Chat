import { authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common.types";
import { pathParam } from "@/shared/api/http-client";
import type {
  PatchUIComponentRequest,
  UIComponentDTO,
  UIComponentData,
  UIComponentDeleteData,
  UIComponentPage,
  WriteUIComponentRequest,
} from "@/shared/api/ui-components.types";

type UIComponentListOptions = {
  query?: string;
  scope?: "builtin" | "platform";
  enabled?: boolean;
  page?: number;
  pageSize?: number;
};

function listPath(basePath: string, options: UIComponentListOptions = {}): string {
  const params = new URLSearchParams({
    page: String(options.page ?? 1),
    page_size: String(options.pageSize ?? 100),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (options.scope) params.set("scope", options.scope);
  if (typeof options.enabled === "boolean") params.set("enabled", String(options.enabled));
  return `${basePath}?${params.toString()}`;
}

function normalizePage(data: PagePayload<UIComponentDTO>): UIComponentPage {
  return { results: data.results ?? [], total: data.total ?? 0 };
}

export async function listVisibleUIComponents(
  accessToken: string,
  options: UIComponentListOptions = {},
  signal?: AbortSignal,
): Promise<UIComponentPage> {
  const data = await authedRequest<PagePayload<UIComponentDTO>>(listPath("/api/v1/ui-components", options), { accessToken, signal }, true);
  return normalizePage(data);
}

export async function listMyUIComponents(accessToken: string, options: UIComponentListOptions = {}): Promise<UIComponentPage> {
  const data = await authedRequest<PagePayload<UIComponentDTO>>(listPath("/api/v1/ui-components/mine", options), { accessToken }, true);
  return normalizePage(data);
}

export async function createMyUIComponent(accessToken: string, payload: WriteUIComponentRequest): Promise<UIComponentData> {
  return authedRequest<UIComponentData>("/api/v1/ui-components/mine", { method: "POST", accessToken, body: payload }, true);
}

export async function updateMyUIComponent(accessToken: string, id: number, payload: PatchUIComponentRequest): Promise<UIComponentData> {
  return authedRequest<UIComponentData>(`/api/v1/ui-components/mine/${pathParam(id)}`, { method: "PATCH", accessToken, body: payload }, true);
}

export async function deleteMyUIComponent(accessToken: string, id: number): Promise<UIComponentDeleteData> {
  return authedRequest<UIComponentDeleteData>(`/api/v1/ui-components/mine/${pathParam(id)}`, { method: "DELETE", accessToken }, true);
}

export async function listAdminUIComponents(
  accessToken: string,
  options: UIComponentListOptions = {},
  signal?: AbortSignal,
): Promise<UIComponentPage> {
  const data = await authedRequest<PagePayload<UIComponentDTO>>(listPath("/api/v1/admin/ui-components", options), { accessToken, signal }, true);
  return normalizePage(data);
}

export async function createAdminUIComponent(accessToken: string, payload: WriteUIComponentRequest): Promise<UIComponentData> {
  return authedRequest<UIComponentData>("/api/v1/admin/ui-components", { method: "POST", accessToken, body: payload }, true);
}

export async function updateAdminUIComponent(accessToken: string, id: number, payload: PatchUIComponentRequest): Promise<UIComponentData> {
  return authedRequest<UIComponentData>(`/api/v1/admin/ui-components/${pathParam(id)}`, { method: "PATCH", accessToken, body: payload }, true);
}

export async function deleteAdminUIComponent(accessToken: string, id: number): Promise<UIComponentDeleteData> {
  return authedRequest<UIComponentDeleteData>(`/api/v1/admin/ui-components/${pathParam(id)}`, { method: "DELETE", accessToken }, true);
}
