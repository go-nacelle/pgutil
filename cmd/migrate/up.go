package main

import (
	"context"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Run all schema migrations",
	Long:  `TODO`,
	RunE:  up,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func up(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runner, err := runner(ctx)
	if err != nil {
		return err
	}

	return runner.Run(ctx)
}
