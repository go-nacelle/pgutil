package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/pgutil"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe the current database schema",
	Long:  `TODO`,
	RunE:  describe,
}

func init() {
	rootCmd.AddCommand(describeCmd)
}

func describe(cmd *cobra.Command, args []string) error {
	url := "postgres://efritz@localhost:5432/efritz?sslmode=disable" // TODO
	logger := nacelle.NewNilLogger()                                 // TODO

	db, err := pgutil.Dial(url, logger)
	if err != nil {
		return err
	}

	ctx := context.Background()
	description, err := pgutil.DescribeSchema(ctx, db)
	if err != nil {
		return err
	}

	serialized, err := json.Marshal(description)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", serialized)
	return nil
}
