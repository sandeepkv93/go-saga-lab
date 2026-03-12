package postgres

import (
	"context"
	"testing"
)

func TestNewRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty database URL")
	}
}
