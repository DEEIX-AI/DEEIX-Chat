package personalprovider

import (
	"encoding/json"
	"testing"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
)

func TestResponsesCarryTheIconButNeverTheKey(t *testing.T) {
	item := domainpersonalprovider.Provider{PublicID: "abc123def456", OwnerUserID: 7, Name: "Relay", Icon: "deepseek", APIKeyEnc: "v1:secret", KeyHint: "sk-••••1234"}

	user := toProviderResponse(item)
	if user.Icon != "deepseek" || user.KeyHint != "sk-••••1234" {
		t.Fatalf("user response = %#v", user)
	}

	admin := toAdminProviderResponses([]domainpersonalprovider.Provider{item}, map[uint]appadmin.UserLabel{
		7: {ID: 7, PublicID: "u7", Username: "alice", DisplayName: "Alice", Email: "alice@example.com", Label: "Alice"},
	})
	if len(admin) != 1 || admin[0].Icon != "deepseek" || admin[0].OwnerDisplayName != "Alice" || admin[0].OwnerEmail != "alice@example.com" || admin[0].OwnerPublicID != "u7" {
		t.Fatalf("admin response = %#v", admin)
	}
}

// 旧客户端（包括一键导入）只提交模型名；新客户端提交模型名与协议。
func TestModelRequestAcceptsNamesAndObjects(t *testing.T) {
	var req CreatePersonalProviderRequest
	body := `{"protocol":"openai_chat_completions","baseURL":"https://api.example.com/v1","apiKey":"sk-x","models":["gpt-4o",{"name":"gpt-image-1","protocols":["openai_image_generations","openai_image_edits"]}]}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	inputs := toModelInputs(req.Models)
	if len(inputs) != 2 || inputs[0].Name != "gpt-4o" || len(inputs[0].Protocols) != 0 ||
		inputs[1].Name != "gpt-image-1" || len(inputs[1].Protocols) != 2 {
		t.Fatalf("inputs = %#v", inputs)
	}
}
