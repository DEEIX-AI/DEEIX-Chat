import type {
  PatchUIComponentRequest as ContractPatchUIComponentRequest,
  UIComponentDataResponse,
  UIComponentDeleteDataResponse,
  UIComponentPageResponseDoc,
  UIComponentResponse,
  WriteUIComponentRequest as ContractWriteUIComponentRequest,
} from "@deeix/api-contract";

export type UIComponentScope = "builtin" | "platform" | "user";
export type UIComponentRendererKind = "builtin" | "sandbox";

export type UIComponentDTO = Omit<UIComponentResponse, "scope" | "rendererKind"> & {
  scope: UIComponentScope;
  rendererKind: UIComponentRendererKind;
};

type ContractPage = UIComponentPageResponseDoc["data"];

export type UIComponentPage = Omit<ContractPage, "results"> & {
  results: UIComponentDTO[];
};

export type WriteUIComponentRequest = ContractWriteUIComponentRequest;
export type PatchUIComponentRequest = ContractPatchUIComponentRequest;

export type UIComponentData = Omit<UIComponentDataResponse, "component"> & {
  component: UIComponentDTO;
};

export type UIComponentDeleteData = UIComponentDeleteDataResponse;
