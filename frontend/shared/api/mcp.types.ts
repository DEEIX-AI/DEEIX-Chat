export type MCPToolDTO = {
  id: number;
  serverID: number;
  serverName: string;
  name: string;
  displayName: string;
  description: string;
  inputSchemaJSON: string;
  status: string;
  createdAt: string;
  updatedAt: string;
};

export type MCPToolListResponse = {
  results: MCPToolDTO[];
};
