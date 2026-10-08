package personalprovider

import (
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
