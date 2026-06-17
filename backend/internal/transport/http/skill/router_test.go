package skill

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/skill"
	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type routeSkillRepo struct {
	items []domainskill.Skill
}

func (r routeSkillRepo) ListSkills(context.Context, repository.SkillListFilter, int, int) ([]domainskill.Skill, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r routeSkillRepo) GetSkill(context.Context, uint) (*domainskill.Skill, error) {
	return nil, repository.ErrNotFound
}

func (r routeSkillRepo) CreateSkill(context.Context, *domainskill.Skill) (*domainskill.Skill, error) {
	return nil, repository.ErrInvalidInput
}

func (r routeSkillRepo) PatchSkill(context.Context, uint, repository.SkillPatch) (*domainskill.Skill, error) {
	return nil, repository.ErrInvalidInput
}

func (r routeSkillRepo) DeleteSkill(context.Context, uint) error {
	return repository.ErrInvalidInput
}

func TestSkillMineRouteIsNotCapturedByVisibleDetailRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uint(7))
		c.Next()
	})

	module := NewModule(NewHandler(appskill.NewService(routeSkillRepo{
		items: []domainskill.Skill{{
			ID:          1,
			Scope:       domainskill.ScopeUser,
			OwnerUserID: 7,
			Title:       "Review",
			Trigger:     "review",
			Markdown:    "private SKILL.md",
			Enabled:     true,
		}},
	})))
	module.RegisterRoutes(router.Group(""))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/skills/mine", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected /skills/mine to return 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "private SKILL.md") {
		t.Fatalf("expected mine route to return full skill payload, got %s", recorder.Body.String())
	}
}
