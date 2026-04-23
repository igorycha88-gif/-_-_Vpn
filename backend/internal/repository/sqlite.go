package repository

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string, migrationsFS fs.FS) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("repository.InitDB: open: %w", err)
	}

	db.SetMaxOpenConns(1)

	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("repository.InitDB: set WAL: %w", err)
	}

	_, err = db.Exec("PRAGMA foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("repository.InitDB: set FK: %w", err)
	}

	if err := runMigrations(db, migrationsFS); err != nil {
		return nil, fmt.Errorf("repository.InitDB: migrations: %w", err)
	}

	slog.Info("база данных инициализирована", "path", dbPath)
	return db, nil
}

func runMigrations(db *sql.DB, migrationsFS fs.FS) error {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") && e.Name() != "embed.go" {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, name := range sqlFiles {
		content, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		_, err = db.Exec(string(content))
		if err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				slog.Info("миграция пропущена (колонка уже существует)", "file", name)
				continue
			}
			return fmt.Errorf("exec migration %s: %w", name, err)
		}

		slog.Info("миграция применена", "file", name)
	}

	return nil
}
