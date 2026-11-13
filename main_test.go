package pgqueue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

var (
	conn *pgx.Conn
	ctx  context.Context
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run holds the whole test setup. It returns the exit code instead of calling
// os.Exit itself, so the deferred calls below do run.
func run(m *testing.M) int {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
		return 1
	}

	// Parse first, so the guard can reject the database before any connection exists.
	dbConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to parse DATABASE_URL: %v\n", err)
		return 1
	}
	if !strings.HasSuffix(dbConfig.Database, "_test") {
		fmt.Fprintf(os.Stderr, "refusing to run: database %q does not end in _test\n", dbConfig.Database)
		return 1
	}

	ctx = context.Background()
	conn, err = pgx.ConnectConfig(ctx, dbConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to database: %v\n", err)
		return 1
	}
	defer conn.Close(ctx)

	// Clean state: remove everything a previous run left behind.
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS pgqueue CASCADE"); err != nil {
		fmt.Fprintf(os.Stderr, "unable to drop schema: %v\n", err)
		return 1
	}

	// TODO (next session):
	// 1. create migrations/001_create_jobs_table.sql
	// 2. embed it and apply the files here, in name order
	// 3. keep conn in a package-level variable, for the per-test helper
	return m.Run()
}

func TestJobTableExists(t *testing.T) {
	t.Run("Test job table exists", func(t *testing.T) {
		exists, err := tableExists(ctx, conn, "pgqueue.jobs")
		if err != nil {
			t.Fatalf("tableExists(%q) failed: %v", "pgqueue.jobs", err)
		}
		if !exists {
			t.Errorf("table %q does not exist", "pgqueue.jobs")
		}
	})
}

// tableExists reports whether the named table exists. It uses to_regclass,
// which returns NULL for a missing table instead of raising an error and
// accepts a schema-qualified name such as "pgqueue.jobs".
func tableExists(ctx context.Context, conn *pgx.Conn, tableName string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, "select to_regclass($1) is not null", tableName).Scan(&exists)
	return exists, err
}
