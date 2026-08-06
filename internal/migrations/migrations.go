package migrations

import (
	"embed"
	"fmt"
	"gorm.io/gorm"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var files embed.FS

func Apply(db *gorm.DB) error {
	if err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())").Error; err != nil {
		return err
	}
	entries, err := fs.ReadDir(files, "sql")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		version, err := strconv.ParseInt(strings.SplitN(entry.Name(), "_", 2)[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid migration %s", entry.Name())
		}
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		sql, err := files.ReadFile("sql/" + entry.Name())
		if err != nil {
			return err
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(sql)).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
