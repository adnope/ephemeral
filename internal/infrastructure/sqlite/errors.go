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
	const attempts = 3

	var err error
	for attempt := range attempts {
		err = operation()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil || !isSQLiteInterrupt(err) {
			return err
		}

		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
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
