package dberror

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

type sqlStateError interface {
	SQLState() string
}

// IsRecordNotFound 判断底层错误是否表示记录不存在。
func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsUniqueConstraint 判断底层错误是否表示唯一约束冲突。
func IsUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var stateErr sqlStateError
	if errors.As(err, &stateErr) && stateErr.SQLState() == "23505" {
		return true
	}
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code != sqlite3.ErrConstraint {
		return false
	}
	switch sqliteErr.ExtendedCode {
	case sqlite3.ErrConstraintUnique, sqlite3.ErrConstraintPrimaryKey, sqlite3.ErrConstraintRowID:
		return true
	default:
		return false
	}
}

// Translate 将通用数据库错误映射为仓储契约错误。
func Translate(err error) error {
	if err == nil {
		return nil
	}
	if IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	if IsUniqueConstraint(err) {
		return repository.ErrDuplicate
	}
	return err
}
