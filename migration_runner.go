package pgutil

import (
	"context"
	"errors"
	"time"

	"github.com/go-nacelle/nacelle"
	"github.com/jackc/pgconn"
)

type Runner struct {
	db     DB
	reader MigrationReader
	logger nacelle.Logger
	locker *TransactionalLocker
}

// TODO - log more/better

// TODO - rename
func NewRunner(db DB, reader MigrationReader, logger nacelle.Logger) (*Runner, error) {
	locker, err := NewTransactionalLocker(db, StringKey("nacelle/pgutil.migration-runner"))
	if err != nil {
		return nil, err
	}

	return &Runner{
		db:     db,
		reader: reader,
		logger: logger,
		locker: locker,
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	// TODO - do outside of the runner?
	definitions, err := ReadMigrations(r.reader)
	if err != nil {
		return err
	}

	// TODO - outside of runner?
	if err := r.EnsureMigrationLogsTable(ctx); err != nil {
		return err
	}

	for {
		if upToDate, err := r.applyDefinitions(ctx, definitions); err != nil {
			return err
		} else if upToDate {
			return nil
		}
	}
}

func (r *Runner) EnsureMigrationLogsTable(ctx context.Context) error {
	for _, query := range []string{
		"CREATE TABLE IF NOT EXISTS migration_logs(id SERIAL PRIMARY KEY)",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS migration_id integer NOT NULL",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS reverse bool NOT NULL",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS started_at timestamptz NOT NULL DEFAULT current_timestamp",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS last_heartbeat_at timestamptz",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS finished_at timestamptz",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS success boolean",
		"ALTER TABLE migration_logs ADD COLUMN IF NOT EXISTS error_message text",
	} {
		if err := r.db.Exec(ctx, RawQuery(query)); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) applyDefinitions(ctx context.Context, definitions []Definition) (upToDate bool, _ error) {
	var cicDefinition *Definition

	if err := r.locker.WithLock(ctx, StringKey("ddl"), func(_ DB) (err error) {
		upToDate, cicDefinition, err = r.applyDefinitionsLocked(ctx, definitions)
		return err
	}); err != nil {
		return false, err
	}

	if cicDefinition != nil {
		if err := r.applyConcurrentIndexCreation(ctx, *cicDefinition); err != nil {
			return false, err
		}

		return false, nil
	}

	return upToDate, nil
}

func (r *Runner) applyDefinitionsLocked(ctx context.Context, definitions []Definition) (upToDate bool, cicDefinition *Definition, _ error) {
	migrationLogs, err := r.MigrationLogs(ctx)
	if err != nil {
		return false, nil, err
	}

	//
	// TODO - need to store hash as well?
	//

	applied := map[int]struct{}{}
	for _, log := range migrationLogs {
		if log.Success != nil && *log.Success && !log.Reverse {
			applied[log.MigrationID] = struct{}{}
		}
	}

	var migrationsToApply []Definition
	for _, definition := range definitions {
		if _, ok := applied[definition.ID]; !ok {
			migrationsToApply = append(migrationsToApply, definition)
		}
	}

	if len(migrationsToApply) == 0 {
		return true, nil, nil
	}

	for _, definition := range migrationsToApply {
		if definition.IndexMetadata != nil {
			return false, &definition, nil
		}

		if err := r.applyDefinition(ctx, definition, false); err != nil {
			return false, nil, err
		}
	}

	return upToDate, cicDefinition, nil
}

func (r *Runner) applyDefinition(ctx context.Context, definition Definition, reverse bool) (err error) {
	return r.withMigrationLog(ctx, definition, reverse, func(_ int) error {
		return r.db.WithTransaction(ctx, func(tx DB) error {
			query := definition.UpQuery
			if reverse {
				query = definition.DownQuery
			}

			return tx.Exec(ctx, query)
		})
	})
}

func (r *Runner) applyConcurrentIndexCreation(ctx context.Context, definition Definition) error {
	tableName := definition.IndexMetadata.TableName
	indexName := definition.IndexMetadata.IndexName

indexPollLoop:
	for i := 0; ; i++ {
		if i != 0 {
			if err := wait(ctx, time.Second*5); err != nil {
				return err
			}
		}

		indexStatus, exists, err := r.getIndexStatus(ctx, tableName, indexName)
		if err != nil {
			return err
		}

		if exists {
			if indexStatus.IsValid {
				if recheck, err := r.handleValidIndex(ctx, definition); err != nil {
					return err
				} else if recheck {
					continue indexPollLoop
				}
			}

			if indexStatus.Phase != nil {
				continue indexPollLoop
			}

			if err := r.db.Exec(ctx, Query("DROP INDEX IF EXISTS {:indexName}", Args{
				"indexName": indexName,
			})); err != nil {
				return err
			}
		}

		if raceDetected, err := r.createIndexConcurrently(ctx, definition); err != nil {
			return err
		} else if raceDetected {
			continue indexPollLoop
		}

		return nil
	}
}

func (r *Runner) createIndexConcurrently(ctx context.Context, definition Definition) (raceDetected bool, _ error) {
	err := r.withMigrationLog(ctx, definition, false, func(id int) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			for {
				if err := r.db.Exec(ctx, Query(`
					UPDATE migration_logs
					SET last_heartbeat_at = current_timestamp
					WHERE id = {:id}
				`, Args{
					"id": id,
				})); err != nil {
					r.logger.Error("Failed to update heartbeat") // TODO - more information
				}

				if err := wait(ctx, time.Second*5); err != nil {
					break
				}
			}
		}()

		if err := r.db.Exec(ctx, definition.UpQuery); err != nil {
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code == "42P07" {
				return err
			}

			if err := r.db.Exec(ctx, Query("DELETE FROM migration_logs WHERE id = {:id}", Args{"id": id})); err != nil {
				return err
			}

			raceDetected = true
		}

		return nil
	})

	return raceDetected, err
}

func (r *Runner) handleValidIndex(ctx context.Context, definition Definition) (recheck bool, _ error) {
	err := r.locker.WithLock(ctx, StringKey("log"), func(tx DB) error {
		log, ok, err := r.getLogForConcurrentIndex(ctx, tx, definition.ID)
		if err != nil {
			return err
		}

		if !ok {
			if err := tx.Exec(ctx, Query(`
				INSERT INTO migration_logs (migration_id, reverse, finished_at, success)
				VALUES ({:id}, false, current_timestamp, true)
			`, Args{
				"id": definition.ID,
			})); err != nil {
				return err
			}

			return nil
		}

		if log.Success != nil {
			if *log.Success {
				return nil
			}

			return errors.New(*log.ErrorMessage)
		}

		if time.Since(log.LastHeartbeatAt) >= time.Second*15 {
			recheck = true
			return nil
		}

		if err := tx.Exec(ctx, Query(`
			UPDATE migration_logs
			SET success = true, finished_at = current_timestamp
			WHERE id = {:id}
		`, Args{
			"id": log.ID,
		})); err != nil {
			return err
		}

		return nil
	})

	return recheck, err
}

//
//

type migrationLog struct {
	MigrationID int
	Reverse     bool
	Success     *bool
}

var scanMigrationLogs = NewSliceScanner(func(s Scanner) (ms migrationLog, _ error) {
	err := s.Scan(&ms.MigrationID, &ms.Reverse, &ms.Success)
	return ms, err
})

func (r *Runner) MigrationLogs(ctx context.Context) (map[int]migrationLog, error) {
	migrationLogs, err := scanMigrationLogs(r.db.Query(ctx, RawQuery(`
		WITH ranked_migration_logs AS (
			SELECT
				l.*,
				ROW_NUMBER() OVER (PARTITION BY migration_id ORDER BY started_at DESC) AS rank
			FROM migration_logs l
		)
		SELECT
			migration_id,
			reverse,
			success
		FROM ranked_migration_logs
		WHERE rank = 1
		ORDER BY migration_id
	`)))
	if err != nil {
		return nil, err
	}

	logMap := map[int]migrationLog{}
	for _, state := range migrationLogs {
		logMap[state.MigrationID] = state
	}

	return logMap, nil
}

type indexStatus struct {
	IsValid      bool
	Phase        *string
	LockersTotal *int
	LockersDone  *int
	BlocksTotal  *int
	BlocksDone   *int
	TuplesTotal  *int
	TuplesDone   *int
}

var scanIndexStatus = NewFirstScanner(func(s Scanner) (is indexStatus, _ error) {
	err := s.Scan(
		&is.IsValid,
		&is.Phase,
		&is.LockersTotal,
		&is.LockersDone,
		&is.BlocksTotal,
		&is.BlocksDone,
		&is.TuplesTotal,
		&is.TuplesDone,
	)
	return is, err
})

func (r *Runner) getIndexStatus(ctx context.Context, tableName, indexName string) (indexStatus, bool, error) {
	return scanIndexStatus(r.db.Query(ctx, Query(`
		SELECT
			index.indisvalid,
			progress.phase,
			progress.lockers_total,
			progress.lockers_done,
			progress.blocks_total,
			progress.blocks_done,
			progress.tuples_total,
			progress.tuples_done
		FROM pg_catalog.pg_class table_class
		JOIN pg_catalog.pg_index index ON index.indrelid = table_class.oid
		JOIN pg_catalog.pg_class index_class ON index_class.oid = index.indexrelid
		LEFT JOIN pg_catalog.pg_stat_progress_create_index progress ON progress.relid = table_class.oid AND progress.index_relid = index_class.oid
		WHERE
			table_class.relname = {:tableName} AND
			index_class.relname = {:indexName}
	`, Args{
		"tableName": tableName,
		"indexName": indexName,
	})))
}

type concurrentIndexLog struct {
	ID              int
	Success         *bool
	ErrorMessage    *string
	LastHeartbeatAt time.Time
}

var scanConcurrentIndexLog = NewFirstScanner(func(s Scanner) (l concurrentIndexLog, _ error) {
	err := s.Scan(&l.ID, &l.Success, &l.ErrorMessage, &l.LastHeartbeatAt)
	return l, err
})

func (r *Runner) getLogForConcurrentIndex(ctx context.Context, db DB, id int) (concurrentIndexLog, bool, error) {
	return scanConcurrentIndexLog(db.Query(ctx, Query(`
		WITH ranked_migration_logs AS (
			SELECT
				l.*,
				ROW_NUMBER() OVER (ORDER BY started_at DESC) AS rank
			FROM migration_logs l
			WHERE migration_id = {:id}
		)
		SELECT
			id,
			success,
			error_message,
			COALESCE(last_heartbeat_at, started_at)
		FROM ranked_migration_logs
		WHERE rank = 1 AND NOT reverse
	`, Args{
		"id": id,
	})))
}

func (r *Runner) withMigrationLog(ctx context.Context, definition Definition, reverse bool, f func(id int) error) (err error) {
	id, _, err := ScanInt(r.db.Query(ctx, Query(`
		INSERT INTO migration_logs (migration_id, reverse)
		VALUES ({:id}, {:reverse})
		RETURNING id
	`, Args{
		"id":      definition.ID,
		"reverse": reverse,
	})))
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, r.db.Exec(ctx, Query(`
			UPDATE migration_logs
			SET
				finished_at = current_timestamp,
				success = {:success},
				error_message = {:error_message}
			WHERE id = {:id}
		`, Args{
			"success":       err == nil,
			"error_message": extractErrorMessage(err),
			"id":            id,
		})))
	}()

	return f(id)
}

func wait(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func extractErrorMessage(err error) *string {
	if err == nil {
		return nil
	}

	msg := err.Error()
	return &msg
}
