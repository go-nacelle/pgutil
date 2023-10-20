package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage and execute Postgres schema migrations",
	Long:  `TODO`,
}

var (
	migrationDirectory string
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&migrationDirectory, "dir", "d", "migrations", `TODO`)
}
