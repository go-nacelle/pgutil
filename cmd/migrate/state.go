package main

import (
	"context"
	"fmt"

	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/pgutil"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Display the current state of the database schema",
	Long:  `TODO`,
	RunE:  state,
}

func init() {
	rootCmd.AddCommand(stateCmd)
}

func state(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runner, err := runner(ctx)
	if err != nil {
		return err
	}

	logs, err := runner.MigrationLogs(ctx)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Printf("Empty database.\n")
	} else {
		for _, log := range logs {
			fmt.Printf("> %v %v %v\n", log.MigrationID, log.Reverse, log.Success)
		}
	}

	return nil
}

func runner(ctx context.Context) (*pgutil.Runner, error) {
	url := "postgres://efritz@localhost:5432/efritz?sslmode=disable" // TODO
	logger := nacelle.NewNilLogger()                                 // TODO

	db, err := pgutil.Dial(url, logger)
	if err != nil {
		return nil, err
	}

	reader := pgutil.NewFilesystemMigrationReader(migrationDirectory)
	runner, err := pgutil.NewRunner(db, reader, logger)
	if err != nil {
		return nil, err
	}

	if err := runner.EnsureMigrationLogsTable(ctx); err != nil {
		return nil, err
	}

	return runner, nil
}
