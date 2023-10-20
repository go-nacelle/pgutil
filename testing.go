package pgutil

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/go-nacelle/log"
	"github.com/lib/pq"
)

func NewTestDB(t testing.TB) DB {
	t.Helper()

	id, err := randomHexString(16)
	if err != nil {
		t.Fatalf("failed to generate random id (%s)", err)
	}

	var (
		url                  = "postgres://efritz@localhost:5432/efritz?sslmode=disable" // TODO - configure
		testDatabaseName     = fmt.Sprintf("test-%s", id)                                // TODO - better
		templateDatabaseName = "template0"                                               // TODO - configure
	)

	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("failed to connect to database (%s)", err)
	}

	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", pq.QuoteIdentifier(testDatabaseName), pq.QuoteIdentifier(templateDatabaseName))); err != nil {
		t.Fatalf("failed to create test database (%s)", err)
	}

	url2 := fmt.Sprintf("postgres://efritz@localhost:5432/%s?sslmode=disable", testDatabaseName) // TODO
	testDB, err := sql.Open("postgres", url2)
	if err != nil {
		t.Fatalf("failed to connect to test database (%s)", err)
	}

	t.Cleanup(func() {
		defer db.Close()

		// TODO - leave in-tact if flag exists?
		if false {
			return
		}

		if err := testDB.Close(); err != nil {
			t.Fatalf("failed to close test database (%s)", err)
		}

		if _, err := db.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", testDatabaseName); err != nil {
			t.Fatalf("failed to create test database (%s)", err)
		}

		if _, err := db.Exec(fmt.Sprintf("DROP DATABASE %s", pq.QuoteIdentifier(testDatabaseName))); err != nil {
			t.Fatalf("failed to drop test database (%s)", err)
		}
	})

	// TODO - test logger?
	return newLoggingDB(testDB, log.NewNilLogger())
}
