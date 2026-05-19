package sqlite

import (
	"context"
	"errors"
	"strings"
	"time"

	moderncsqlite "modernc.org/sqlite"
)

const sqliteInterruptCode = 9

func retryInterruptedRead(ctx context.Context, operation func() error) error {
	const attempts = 5

	var err error
	for attempt := range attempts {
		err = operation()
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if !isSQLiteInterrupt(err) {
			return err
		}
		if attempt == attempts-1 {
			return err
		}

		delay := time.Duration(1<<attempt) * 5 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return err
}

func isSQLiteInterrupt(err error) bool {
	var sqliteErr *moderncsqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteInterruptCode {
		return true
	}

	return strings.Contains(err.Error(), "interrupted (9)")
}
