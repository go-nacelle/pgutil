package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/pgutil"
	"github.com/spf13/cobra"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "TODO",
	Long:  `TODO`,
	RunE:  drift,
}

func init() {
	rootCmd.AddCommand(driftCmd)
}

func drift(cmd *cobra.Command, args []string) error {
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

	b, err := os.ReadFile("description.json")
	if err != nil {
		return err
	}

	var expected pgutil.SchemaDescription
	if err := json.Unmarshal(b, &expected); err != nil {
		return err
	}

	for _, d := range pgutil.Compare(expected, description) {
		fmt.Printf("%s\n", d)
	}

	return nil
}
