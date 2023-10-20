package pgutil

import (
	"context"
	"testing"
)

func TestLocker(t *testing.T) {
	var (
		db  = NewTestDB(t)
		ctx = context.Background()
	)

	locker, err := NewTransactionalLocker(db, StringKey("test"))
	if err != nil {
		t.Fatalf("failed to create locker (%s)", err)
	}

	t.Run("sequential", func(t *testing.T) {
		if err := locker.WithLock(ctx, 125, func(tx DB) error {
			return nil
		}); err != nil {
			t.Fatalf("failed to run function with lock (%s)", err)
		}

		if err := locker.WithLock(ctx, 125, func(tx DB) error {
			return nil
		}); err != nil {
			t.Fatalf("failed to run function with lock (%s)", err)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		runWithHeldLock := func(f func()) {
			var (
				signal = make(chan struct{}) // closed when key=125 is acquired
				block  = make(chan struct{}) // closed when key=125 should be released
				errors = make(chan error, 1) // holds acquisition error from goroutine
			)

			go func() {
				defer close(errors)

				if err := locker.WithLock(ctx, 125, func(tx DB) error {
					close(signal)
					<-block
					return nil
				}); err != nil {
					errors <- err
				}
			}()

			<-signal     // Wait for key=125 to be acquired by goroutine above
			f()          // Run test function with held lock
			close(block) // Unblock test routine

			for err := range errors {
				t.Fatalf("failed to run function with lock (%s)", err)
			}
		}

		runWithHeldLock(func() {
			// Test acquisition of concurrently held lock
			if acquired, err := locker.TryWithLock(ctx, 125, func(tx DB) error {
				return nil
			}); err != nil {
				t.Fatalf("failed to run function with lock (%s)", err)
			} else if acquired {
				t.Fatalf("expected lock acquisition to fail; held by another goroutine")
			}

			// Test acquisition of concurrently un-held lock
			if acquired, err := locker.TryWithLock(ctx, 126, func(tx DB) error {
				return nil
			}); err != nil {
				t.Fatalf("failed to run function with lock (%s)", err)
			} else if !acquired {
				t.Fatalf("expected lock to be acquirable")
			}
		})

		// Test acquisition of released lock
		if acquired, err := locker.TryWithLock(ctx, 125, func(tx DB) error {
			return nil
		}); err != nil {
			t.Fatalf("failed to run function with lock (%s)", err)
		} else if !acquired {
			t.Fatalf("expected lock to be acquirable")
		}
	})
}
