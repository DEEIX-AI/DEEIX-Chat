import type {
  CreateServerRequest,
  ReorderServerOrderItem,
  ServerDataResponse,
  ServerListResponse,
  ServerResponse,
  ServerToolOrderListResponse,
  ServerToolOrderResponse,
  ToolListResponse,
  UpdateToolRequest,
} from "@deeix/api-contract";

export type AdminMCPServerDTO = ServerResponse;
export type AdminMCPServerPayload = CreateServerRequest;
export type AdminMCPServerListResponse = ServerListResponse;
export type AdminMCPServerDataResponse = ServerDataResponse;
export type AdminMCPToolListResponse = ToolListResponse;
export type AdminMCPToolPayload = UpdateToolRequest;
export type AdminMCPOrderItemPayload = ReorderServerOrderItem;
export type AdminMCPOrderGroupDTO = ServerToolOrderResponse;
export type AdminMCPOrderListResponse = ServerToolOrderListResponse;
