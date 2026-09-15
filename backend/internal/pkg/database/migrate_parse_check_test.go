package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// lazyConnector 永不真实建连（解析期不应触库；若触库则报错暴露）
type lazyConnector struct{}

func (lazyConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("offline parse check: no real connection allowed")
}
func (lazyConnector) Driver() driver.Driver { return nil }

// 临时校验：全部迁移能被 goose 解析（注解/语句切分无误）。仅解析不执行，无需真实库。
func TestMigrationsParse(t *testing.T) {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(lazyConnector{})
	p, err := goose.NewProvider(goose.DialectPostgres, db, sub)
	if err != nil {
		t.Fatalf("迁移解析失败: %v", err)
	}
	srcs := p.ListSources()
	t.Logf("共 %d 个迁移，最新 %v", len(srcs), srcs[len(srcs)-1])
}
