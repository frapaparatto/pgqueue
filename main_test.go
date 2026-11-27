package pgqueue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/frapaparatto/pgqueue/migrations"
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
	defer conn.Close(ctx) //nolint:errcheck // releases the socket only, every write has already completed

	// Clean state: remove everything a previous run left behind.
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS pgqueue CASCADE"); err != nil {
		fmt.Fprintf(os.Stderr, "unable to drop schema: %v\n", err)
		return 1
	}

	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to list migrations: %v\n", err)
		return 1
	}

	for _, entry := range entries {
		content, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to read migration %s: %v\n", entry.Name(), err)
			return 1
		}

		if _, err := conn.Exec(ctx, string(content)); err != nil {
			fmt.Fprintf(os.Stderr, "unable to apply migration %s: %v\n", entry.Name(), err)
			return 1
		}
	}
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

func TestResetJobs(t *testing.T) {
	resetJobs(t)

	if _, err := conn.Exec(ctx, "INSERT INTO pgqueue.jobs (kind, max_attempts) VALUES ('test', 1)"); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	resetJobs(t)

	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pgqueue.jobs").Scan(&count); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count != 0 {
		t.Errorf("jobs count = %d, want 0", count)
	}

	var id int64
	if err := conn.QueryRow(ctx, "INSERT INTO pgqueue.jobs (kind, max_attempts) VALUES ('test', 1) RETURNING id").Scan(&id); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if id != 1 {
		t.Errorf("id of the first job after reset = %d, want 1", id)
	}
}

// resetJobs empties pgqueue.jobs and restarts its identity, so the calling
// test starts from a clean state regardless of what ran before it.
func resetJobs(t *testing.T) {
	t.Helper()
	if _, err := conn.Exec(ctx, "TRUNCATE pgqueue.jobs RESTART IDENTITY"); err != nil {
		t.Fatalf("reset jobs: %v", err)
	}
}

// tableExists reports whether the named table exists. It uses to_regclass,
// which returns NULL for a missing table instead of raising an error and
// accepts a schema-qualified name such as "pgqueue.jobs".
func tableExists(ctx context.Context, conn *pgx.Conn, tableName string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, "select to_regclass($1) is not null", tableName).Scan(&exists)
	return exists, err
}
