package subscription

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepository_ActiveByUser_FiltersExpiredSubscriptions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://meme:meme@localhost:5432/meme_portal?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skip("skipping db test: postgres pool init error:", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skip("skipping db test: postgres unreachable:", err)
	}

	repo := NewPostgresRepository(pool)

	// Create temporary dummy user and expired subscription
	var userID, planID, subID string
	err = pool.QueryRow(ctx, "INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id",
		"tkt101_test_"+time.Now().Format("20060102150405.000000")+"@example.com", "TKT101 Test", "hash").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	}()

	err = pool.QueryRow(ctx, "SELECT id FROM plans WHERE active = true LIMIT 1").Scan(&planID)
	if err != nil {
		t.Fatalf("failed to get active plan: %v", err)
	}

	// Insert active subscription with ends_at in the past (1 hour ago)
	err = pool.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at)
		VALUES ($1, $2, 'active', now() - interval '2 days', now() - interval '1 hour')
		RETURNING id`,
		userID, planID,
	).Scan(&subID)
	if err != nil {
		t.Fatalf("failed to insert expired subscription: %v", err)
	}

	// ActiveByUser should consider it expired and return ErrNotSubscribed
	_, _, err = repo.ActiveByUser(ctx, userID)
	if err != ErrNotSubscribed {
		t.Fatalf("expected ErrNotSubscribed for subscription expired 1 hour ago, got %v", err)
	}
}
