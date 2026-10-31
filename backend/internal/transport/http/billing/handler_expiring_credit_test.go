package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func TestGrantExpiringCreditRejectsInvalidUserID(t *testing.T) {
	for _, id := range []string{"0", "-1", "invalid", "18446744073709551616"} {
		t.Run(id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("POST", "/admin/billing/accounts/"+id+"/expiring-credits", nil)
			ctx.Params = gin.Params{{Key: "user_id", Value: id}}
			(&Handler{}).GrantExpiringCredit(ctx)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
			var body struct {
				ErrorCode string `json:"errorCode"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.ErrorCode == "" {
				t.Fatalf("missing error code: %s", recorder.Body)
			}
		})
	}
}

func TestGrantExpiringCreditRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"creditUSD":20,"durationDays":0.1,"idempotencyKey":"grant-0001"}`, true},
		{`{"creditUSD":20,"durationDays":3650,"idempotencyKey":"grant-0001"}`, true},
		{`{"creditUSD":0,"durationDays":1,"idempotencyKey":"grant-0001"}`, false},
		{`{"creditUSD":20,"durationDays":0.05,"idempotencyKey":"grant-0001"}`, false},
		{`{"creditUSD":20,"durationDays":3651,"idempotencyKey":"grant-0001"}`, false},
		{`{"creditUSD":20,"durationDays":1}`, false},
		{`{"creditUSD":20,"idempotencyKey":"grant-0001"}`, false},
	} {
		var req GrantExpiringCreditRequest
		if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
			t.Fatal(err)
		}
		err := binding.Validator.ValidateStruct(req)
		if (err == nil) != tc.valid {
			t.Fatalf("request %s: validation error %v, valid=%v", tc.body, err, tc.valid)
		}
	}
}
