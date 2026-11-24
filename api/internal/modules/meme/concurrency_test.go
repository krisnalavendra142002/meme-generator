package meme

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

func TestRepository_Create_EnforcesQuotaUnderConcurrency(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://meme:meme@localhost:5432/meme_portal?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Create test user
	var userID, planID string
	testEmail := "tkt109_test_" + token.RandomHex(8) + "@example.com"
	err = pool.QueryRow(ctx, "INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id",
		testEmail, "TKT109 Concurrency Test", "hash").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	}()

	// Create custom plan with max_memes = 3
	planCode := "plan_tkt109_" + token.RandomHex(8)
	err = pool.QueryRow(ctx, `
		INSERT INTO plans (code, name, price_idr, duration_days, max_memes, active)
		VALUES ($1, 'Test Plan 3 Memes', 1000, 1, 3, true)
		RETURNING id`, planCode).Scan(&planID)
	if err != nil {
		t.Fatalf("failed to insert test plan: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM plans WHERE id = $1", planID)
	}()

	// Activate subscription
	_, err = pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at)
		VALUES ($1, $2, 'active', now(), now() + interval '1 day')`,
		userID, planID)
	if err != nil {
		t.Fatalf("failed to activate subscription: %v", err)
	}

	// Launch 10 concurrent meme creations (simulating 10 tabs simultaneously)
	// Quota is 3. Only exactly 3 must succeed, 7 must be rejected with ErrNotSubscribedGate.
	const concurrentRequests = 10
	var wg sync.WaitGroup
	var successCount, failureCount int
	var mu sync.Mutex

	startSignal := make(chan struct{})

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startSignal // wait for simultaneous trigger
			_, err := repo.Create(context.Background(), Meme{
				UserID:       userID,
				Title:        "Concurrent Meme",
				BaseImageURL: "http://example.com/base.jpg",
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if err == ErrNotSubscribedGate {
				failureCount++
			} else {
				t.Errorf("unexpected error on goroutine %d: %v", idx, err)
			}
		}(i)
	}

	// Release all goroutines simultaneously
	close(startSignal)
	wg.Wait()

	if successCount != 3 {
		t.Fatalf("expected exactly 3 memes created, got %d (failures: %d)", successCount, failureCount)
	}
	if failureCount != 7 {
		t.Fatalf("expected 7 rejected requests, got %d", failureCount)
	}

	// Check database count
	var dbCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM memes WHERE user_id = $1", userID).Scan(&dbCount)
	if err != nil {
		t.Fatalf("failed to count memes: %v", err)
	}
	if dbCount != 3 {
		t.Fatalf("expected exactly 3 memes in database, got %d", dbCount)
	}
}
