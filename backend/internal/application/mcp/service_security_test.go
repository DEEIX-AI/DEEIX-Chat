package mcp

import "testing"

func TestValidateServerBaseURLAllowsAdministratorConfiguredPrivateOrigin(t *testing.T) {
	service := &Service{}
	if err := service.validateServerBaseURL("http://mcp-server:8080/mcp"); err != nil {
		t.Fatalf("private MCP endpoint rejected: %v", err)
	}
	if err := service.validateServerBaseURL("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("metadata endpoint must remain blocked")
	}
}
