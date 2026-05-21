package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 提供公共字段。
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;comment:主键ID"`
	CreatedAt time.Time      `gorm:"comment:创建时间"`
	UpdatedAt time.Time      `gorm:"comment:更新时间"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:软删除时间"`
}

// ControlPlaneModel 提供仅保留创建/更新时间的控制面主数据基类。
// LLM 管理域对象采用硬删除语义，不参与软删除默认作用域。
type ControlPlaneModel struct {
	ID        uint      `gorm:"primaryKey;comment:主键ID"`
	CreatedAt time.Time `gorm:"comment:创建时间"`
	UpdatedAt time.Time `gorm:"comment:更新时间"`
}
