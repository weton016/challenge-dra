package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL integration")
	}
	if !strings.Contains(url, "/pix_test?") {
		t.Fatal("tests require isolated pix_test database")
	}
	s, err := Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Pool.Close)
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = s.Pool.Exec(context.Background(), `TRUNCATE outbox,entries,operations,external_receipts,accounts CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(context.Background(), 3, 100); err != nil {
		t.Fatal(err)
	}
	return s
}
