package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-nacelle/pgutil"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [flags] 'migration name'",
	Short: "Create a new schema migration",
	Long:  `TODO`,
	Args:  cobra.ExactArgs(1),
	RunE:  create,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func create(cmd *cobra.Command, args []string) error {
	if err := ensureMigrationDirectoryExists(migrationDirectory); err != nil {
		return err
	}

	definitions, err := pgutil.ReadMigrations(pgutil.NewFilesystemMigrationReader(migrationDirectory))
	if err != nil {
		return err
	}

	var lastID int
	if len(definitions) > 0 {
		lastID = definitions[len(definitions)-1].ID
	}

	dirPath := filepath.Join(migrationDirectory, fmt.Sprintf("%d_%s", lastID+1, canonicalize(args[0])))
	upPath := filepath.Join(dirPath, "up.sql")
	downPath := filepath.Join(dirPath, "down.sql")

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile(upPath, nil, os.ModePerm); err != nil {
		return err
	}
	if err := os.WriteFile(downPath, nil, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func ensureMigrationDirectoryExists(migrationDirectory string) error {
	stat, err := os.Stat(migrationDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(migrationDirectory, os.ModePerm); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("supplied migration directory is not a directory")
	}

	return nil
}

var nonNamePattern = regexp.MustCompile(`[^a-z0-9_]+`)

func canonicalize(name string) string {
	return strings.ToLower(nonNamePattern.ReplaceAllString(name, "_"))
}
