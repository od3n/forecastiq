package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/forecastiq/forecastiq/internal/platform/config"
	"github.com/forecastiq/forecastiq/internal/platform/db"
	"github.com/forecastiq/forecastiq/migrations"
)

// cmdMigrate applies or reverses schema migrations.
func cmdMigrate(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: forecastiq migrate up|down <n>|force <version>|status")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch args[0] {
	case "up":
		if err := db.Migrate(migrations.FS, cfg.DatabaseURL, cfg.MigrationsTable, 0); err != nil {
			return err
		}
		fmt.Println("migrations applied")
		return nil
	case "down":
		n := 1
		if len(args) > 1 {
			n, _ = strconv.Atoi(args[1])
		}
		if err := db.Migrate(migrations.FS, cfg.DatabaseURL, cfg.MigrationsTable, -n); err != nil {
			return err
		}
		fmt.Printf("rolled back %d migration(s)\n", n)
		return nil
	case "force":
		version := 1
		if len(args) > 1 {
			version, _ = strconv.Atoi(args[1])
		}
		if err := db.ForceClear(migrations.FS, cfg.DatabaseURL, version); err != nil {
			return err
		}
		fmt.Printf("forced version %d\n", version)
		return nil
	case "status":
		v, dirty, err := db.Status(migrations.FS, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown migrate command %q", args[0])
	}
}
