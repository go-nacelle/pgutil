package pgutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBatchInserter(t *testing.T) {
	var (
		db          = NewTestDB(t)
		numRows     = 100000
		numPayloads = 100
		columns     = []string{"w", "x", "y", "z", "q", "payload"}
	)

	setupTestBatchTable(t, db)
	payloads := createBatchPayloads(t, numPayloads)
	rowValues := createBatchRowValues(t, numRows, payloads)

	expectedValues := make([]any, 0, numRows)
	for _, values := range rowValues {
		expectedValues = append(expectedValues, values[0])
	}

	// Insert rows and assert values of column "w"
	inserter := NewBatchInserter(db, "test", columns)
	runBatchInserter(t, inserter, rowValues)
	assertBatchInsertedValues(t, db, "w", expectedValues)
}

func TestBatchInserterWithOnConflict(t *testing.T) {
	t.Run("do nothing", func(t *testing.T) {
		var (
			db          = NewTestDB(t)
			numRows     = 100000
			numPayloads = 100
			columns     = []string{"w", "x", "y", "z", "q", "payload"}
		)

		setupTestBatchTable(t, db)
		payloads := createBatchPayloads(t, numPayloads)
		rowValues := createBatchRowValues(t, numRows, payloads)

		expectedValues := make([]any, 0, numRows)
		for _, values := range rowValues {
			expectedValues = append(expectedValues, values[0])
		}

		// Insert duplicate rows and assert _unique_ values of column "w"
		inserter := NewBatchInserter(db, "test", columns, WithBatchInserterOnConflict("DO NOTHING"))
		runBatchInserter(t, inserter, append(rowValues, createBatchRowValues(t, numRows/4, payloads)...))
		assertBatchInsertedValues(t, db, "w", expectedValues)
	})

	t.Run("update", func(t *testing.T) {
		var (
			db          = NewTestDB(t)
			numRows     = 100000
			numPayloads = 100
			columns     = []string{"w", "x", "y", "z", "q", "payload"}
		)

		setupTestBatchTable(t, db)
		payloads := createBatchPayloads(t, numPayloads)
		initialRowValues := createBatchRowValues(t, numRows/4, payloads)
		rowValues := createBatchRowValues(t, numRows, payloads)
		inserter := NewBatchInserter(db, "test", columns)
		runBatchInserter(t, inserter, initialRowValues)

		expectedValues := make([]any, 0, numRows)
		for i, values := range rowValues {
			if i < len(initialRowValues) {
				// updated
				expectedValues = append(expectedValues, int64(0))
			} else {
				// not updated
				expectedValues = append(expectedValues, values[1])
			}
		}

		// Insert duplicates for update and assert updated values fo column "x"
		inserter = NewBatchInserter(db, "test", columns, WithBatchInserterOnConflict("(w) DO UPDATE SET x = 0, y = 0, z = 0, q = 0"))
		runBatchInserter(t, inserter, rowValues)
		assertBatchInsertedValues(t, db, "x", expectedValues)
	})
}

func TestBatchInserterWithReturning(t *testing.T) {
	var (
		db          = NewTestDB(t)
		numRows     = 100000
		numPayloads = 100
		columns     = []string{"w", "x", "y", "z", "q", "payload"}
		collector   = NewCollector(NewAnyValueScanner[int]())
	)

	setupTestBatchTable(t, db)
	payloads := createBatchPayloads(t, numPayloads)
	rowValues := createBatchRowValues(t, numRows, payloads)

	expectedValues := make([]int, 0, numRows)
	for i := range rowValues {
		expectedValues = append(expectedValues, i+1)
	}

	// Insert rows and assert scanned serial ids
	inserter := NewBatchInserter(db, "test", columns, WithBatchInserterReturn([]string{"id"}, collector.Scanner()))
	runBatchInserter(t, inserter, rowValues)
	if diff := cmp.Diff(expectedValues, collector.Slice()); diff != "" {
		t.Errorf("unexpected values (-want +got):\n%s", diff)
	}
}

//
//

func setupTestBatchTable(t testing.TB, db DB) {
	t.Helper()
	ctx := context.Background()

	if err := db.Exec(ctx, RawQuery(`
		CREATE TABLE test (
			id      SERIAL,
			w       integer NOT NULL UNIQUE,
			x       integer NOT NULL,
			y       integer NOT NULL,
			z       integer NOT NULL,
			q       integer NOT NULL,
			payload text
		)`,
	)); err != nil {
		t.Fatalf("failed to create test table (%s)", err)
	}
}

func runBatchInserter(t testing.TB, inserter *BatchInserter, rowValues [][]any) {
	t.Helper()
	ctx := context.Background()

	for _, values := range rowValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("failed to insert (%s)", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("failed to flush (%s)", err)
	}
}

func assertBatchInsertedValues(t testing.TB, db DB, columnName string, expectedValues []any) {
	t.Helper()
	ctx := context.Background()

	if values, err := ScanAnys(db.Query(ctx, Query("SELECT {:col} FROM test", Args{"col": Quote(columnName)}))); err != nil {
		t.Fatalf("failed to read test values (%s)", err)
	} else if diff := cmp.Diff(expectedValues, values); diff != "" {
		t.Errorf("unexpected values (-want +got):\n%s", diff)
	}
}

func createBatchPayloads(t testing.TB, n int) []string {
	payloads := make([]string, 0, n)
	for i := 0; i < n; i++ {
		payload, err := randomHexString(128)
		if err != nil {
			t.Fatalf("failed to generate random payload (%s)", err)
		}

		payloads = append(payloads, fmt.Sprintf("payload-%s", payload))
	}

	return payloads
}

func createBatchRowValues(t testing.TB, n int, payloads []string) [][]any {
	values := make([][]any, 0, n)
	for i := 0; i < n; i++ {
		values = append(values, []any{
			int64(i*2 + 1),            // w
			int64(i*2 + 2),            // z
			int64(i*2 + 3),            // y
			int64(i*2 + 4),            // z
			int64(i*2 + 5),            // q
			payloads[i%len(payloads)], // payload
		})
	}

	return values
}
