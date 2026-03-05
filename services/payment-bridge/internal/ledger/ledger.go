package ledger

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// UsageRecord tracks a single API call deduction.
type UsageRecord struct {
	UserAddr       string
	RequestID      string
	TokensConsumed int64
	CostUZRN       int64
	Model          string
	Timestamp      time.Time
}

// Ledger provides atomic off-chain balance management backed by Redis.
type Ledger struct {
	rdb *redis.Client
}

// New creates a new Ledger connected to Redis.
func New(redisAddr string) *Ledger {
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &Ledger{rdb: rdb}
}

// Close closes the Redis connection.
func (l *Ledger) Close() error {
	return l.rdb.Close()
}

// Ping checks Redis connectivity.
func (l *Ledger) Ping(ctx context.Context) error {
	return l.rdb.Ping(ctx).Err()
}

func balanceKey(userAddr string) string { return "balance:" + userAddr }
func usageKey(userAddr string) string   { return "usage:" + userAddr }
func pendingKey() string                { return "pending_settlements" }

// CreditDeposit adds funds to a user's off-chain balance after on-chain deposit.
func (l *Ledger) CreditDeposit(ctx context.Context, userAddr string, amountUZRN int64) (int64, error) {
	newBal, err := l.rdb.IncrBy(ctx, balanceKey(userAddr), amountUZRN).Result()
	if err != nil {
		return 0, fmt.Errorf("credit deposit: %w", err)
	}
	return newBal, nil
}

// GetBalance returns the current off-chain balance for a user.
func (l *Ledger) GetBalance(ctx context.Context, userAddr string) (int64, error) {
	val, err := l.rdb.Get(ctx, balanceKey(userAddr)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

// deductScript atomically deducts if balance >= cost, returns new balance or -1.
var deductScript = redis.NewScript(`
local bal = tonumber(redis.call('GET', KEYS[1]) or '0')
local cost = tonumber(ARGV[1])
if bal < cost then
    return -1
end
return redis.call('DECRBY', KEYS[1], cost)
`)

// Deduct atomically deducts cost from user balance.
// Returns new balance, or error if insufficient funds.
func (l *Ledger) Deduct(ctx context.Context, userAddr string, costUZRN int64) (int64, error) {
	result, err := deductScript.Run(ctx, l.rdb, []string{balanceKey(userAddr)}, costUZRN).Int64()
	if err != nil {
		return 0, fmt.Errorf("deduct: %w", err)
	}
	if result < 0 {
		return 0, fmt.Errorf("insufficient balance for %s (cost: %d uzrn)", userAddr, costUZRN)
	}
	return result, nil
}

// RecordUsage appends a usage record for later batch settlement.
func (l *Ledger) RecordUsage(ctx context.Context, rec *UsageRecord) error {
	entry := fmt.Sprintf("%s|%s|%d|%d|%s|%d",
		rec.UserAddr, rec.RequestID, rec.TokensConsumed,
		rec.CostUZRN, rec.Model, rec.Timestamp.Unix())

	pipe := l.rdb.Pipeline()
	pipe.RPush(ctx, usageKey(rec.UserAddr), entry)
	pipe.SAdd(ctx, pendingKey(), rec.UserAddr)
	_, err := pipe.Exec(ctx)
	return err
}

// GetPendingUsers returns all users with unsettled usage.
func (l *Ledger) GetPendingUsers(ctx context.Context) ([]string, error) {
	return l.rdb.SMembers(ctx, pendingKey()).Result()
}

// DrainUsage removes and returns all usage records for a user (for settlement).
func (l *Ledger) DrainUsage(ctx context.Context, userAddr string) ([]string, error) {
	key := usageKey(userAddr)
	records, err := l.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	pipe := l.rdb.Pipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, pendingKey(), userAddr)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// EstimateCost estimates the ZRN cost for a request based on tokens.
func EstimateCost(inputTokens, maxOutputTokens int64, pricePerMillionTokens int64) int64 {
	totalTokens := inputTokens + maxOutputTokens
	// price is per 1M tokens
	cost := (totalTokens * pricePerMillionTokens) / 1_000_000
	if cost < 1 {
		cost = 1 // minimum 1 uzrn
	}
	return cost
}

// HasSufficientBalance checks if user can afford the estimated cost.
func (l *Ledger) HasSufficientBalance(ctx context.Context, userAddr string, estimatedCost int64) (bool, int64, error) {
	bal, err := l.GetBalance(ctx, userAddr)
	if err != nil {
		return false, 0, err
	}
	return bal >= estimatedCost, bal, nil
}
