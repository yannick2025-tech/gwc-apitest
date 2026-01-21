package apitest

import (
	"context"
	"fmt"
	"time"

	"github.com/yannick2025-tech/gwc-db"
)

// DBCleanupHandler 数据库清理处理器（基于软删除）
type DBCleanupHandler struct {
	adapter db.DBAdapter
}

// NewDBCleanupHandler 创建数据库清理处理器
func NewDBCleanupHandler(adapter db.DBAdapter) *DBCleanupHandler {
	return &DBCleanupHandler{
		adapter: adapter,
	}
}

// Execute 执行清理动作
func (h *DBCleanupHandler) Execute(ctx context.Context, action SetupAction) error {
	switch action.Type {
	case "soft_delete_cleanup":
		return h.softDeleteCleanup(ctx, action.Table, action.Condition)
	case "cleanup":
		// 为了保持兼容性，cleanup 也使用软删除
		return h.softDeleteCleanup(ctx, action.Table, action.Condition)
	case "sql":
		return h.executeSQL(ctx, action.SQL)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// softDeleteCleanup 软删除清理（推荐使用）
func (h *DBCleanupHandler) softDeleteCleanup(ctx context.Context, table, condition string) error {
	if table == "" {
		return fmt.Errorf("table name is required")
	}

	deleteVer := time.Now().UnixNano()

	// 构造软删除更新语句
	// 设置 soft_deleted = 1 和 deleted_at = NOW()
	sql := fmt.Sprintf(
		"UPDATE %s SET soft_deleted = %d, deleted_at = ? WHERE soft_deleted = %d",
		table,
		db.Deleted,
		db.NotDeleted,
	)

	args := []interface{}{time.Now(), deleteVer}

	if condition != "" {
		sql += fmt.Sprintf(" AND %s", condition)
	}

	// 获取 XORM 引擎执行原生 SQL
	if xormAdapter, ok := h.adapter.(interface{ GetEngine() interface{} }); ok {
		engine := xormAdapter.GetEngine()

		// XORM Engine 的 Exec 方法
		type XormEngine interface {
			Exec(sql string, args ...interface{}) (interface{}, error)
		}

		if xormEngine, ok := engine.(XormEngine); ok {
			result, err := xormEngine.Exec(sql, args...)
			if err != nil {
				return fmt.Errorf("soft delete cleanup failed: %w", err)
			}

			// 尝试获取影响行数
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				rows, _ := sqlResult.RowsAffected()
				fmt.Printf("  ✓ Soft deleted %d rows from table '%s'\n", rows, table)
			} else {
				fmt.Printf("  ✓ Soft delete cleanup executed on table '%s'\n", table)
			}

			return nil
		}
	}

	return fmt.Errorf("adapter does not support raw SQL execution")
}

// executeSQL 执行自定义 SQL（高级用法，谨慎使用）
func (h *DBCleanupHandler) executeSQL(ctx context.Context, sqlStr string) error {
	if sqlStr == "" {
		return fmt.Errorf("SQL is required")
	}

	// 方案1: 使用类型断言获取 *xorm.Engine
	if xormAdapter, ok := h.adapter.(*db.XormAdapter); ok {
		engine := xormAdapter.GetEngine()

		// 直接使用 xorm.Engine 的 Exec 方法
		// XORM 的 Exec 签名: Exec(string, ...interface{}) (sql.Result, error)
		result, err := engine.Exec(sqlStr)
		if err != nil {
			return fmt.Errorf("SQL execution failed: %w", err)
		}

		// 获取影响行数
		if rows, err := result.RowsAffected(); err == nil {
			fmt.Printf("  ✓ Custom SQL executed successfully (affected %d rows)\n", rows)
		} else {
			fmt.Printf("  ✓ Custom SQL executed successfully\n")
		}

		return nil
	}

	return fmt.Errorf("adapter does not support raw SQL execution")
}

// MockCleanupHandler 用于测试的 Mock 清理处理器
type MockCleanupHandler struct {
	ExecuteFunc func(ctx context.Context, action SetupAction) error
}

// Execute 执行清理动作
func (m *MockCleanupHandler) Execute(ctx context.Context, action SetupAction) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, action)
	}
	fmt.Printf("  Mock cleanup: %s on table %s\n", action.Type, action.Table)
	return nil
}
