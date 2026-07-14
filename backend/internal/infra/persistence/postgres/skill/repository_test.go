package skill

import (
	"context"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteSkillCleansConversationProjectAssociations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:skill_project_cascade?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite connection: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err = db.AutoMigrate(&model.Skill{}, &model.ConversationProjectSkill{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	skill := model.Skill{
		Scope:       "user",
		OwnerUserID: 1,
		Title:       "Project skill",
		Trigger:     "project-skill",
		Enabled:     true,
	}
	if err = db.Create(&skill).Error; err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err = db.Create(&model.ConversationProjectSkill{ProjectID: 9, SkillID: skill.ID}).Error; err != nil {
		t.Fatalf("create project Skill association: %v", err)
	}

	if err = NewRepo(db).DeleteSkill(context.Background(), skill.ID); err != nil {
		t.Fatalf("DeleteSkill() error = %v", err)
	}

	var associationCount int64
	if err = db.Model(&model.ConversationProjectSkill{}).Where("skill_id = ?", skill.ID).Count(&associationCount).Error; err != nil {
		t.Fatalf("count project Skill associations: %v", err)
	}
	if associationCount != 0 {
		t.Fatalf("project Skill association count = %d, want 0", associationCount)
	}
}
