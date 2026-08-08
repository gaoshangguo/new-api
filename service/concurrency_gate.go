package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const concurrencySlotTTLSeconds = 12 * 60 * 60 // 12h 兜底防泄漏

// acquireConcurrencyScript atomically increments the in-flight counter and
// rejects (decrements back) when the limit is exceeded.
var acquireConcurrencyScript = redis.NewScript(`
local v = redis.call('INCR', KEYS[1])
if v > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
return 1
`)

// AcquireConcurrencySlot borrows one in-flight slot for scope:id with the given
// limit (0 = unlimited). Returns false when over the limit. Redis-backed with
// in-memory atomic fallback.
func AcquireConcurrencySlot(ctx context.Context, scope string, id string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	key := fmt.Sprintf("rl:inflight:%s:%s", scope, id)
	rdb := common.RDB
	if rdb != nil {
		res, err := acquireConcurrencyScript.Run(ctx, rdb, []string{key}, limit, concurrencySlotTTLSeconds).Int()
		if err != nil {
			return false, err
		}
		return res == 1, nil
	}
	// 内存降级：单节点语义
	return memoryConcurrency.acquire(scope, id, limit), nil
}

// ReleaseConcurrencySlot releases one in-flight slot. Idempotent.
func ReleaseConcurrencySlot(ctx context.Context, scope string, id string) error {
	key := fmt.Sprintf("rl:inflight:%s:%s", scope, id)
	rdb := common.RDB
	if rdb != nil {
		_, err := rdb.Decr(ctx, key).Result()
		return err
	}
	memoryConcurrency.release(scope, id)
	return nil
}

type memoryConcurrencyCounter struct {
	mu    sync.Mutex
	count map[string]int64
}

var memoryConcurrency = &memoryConcurrencyCounter{count: map[string]int64{}}

func (m *memoryConcurrencyCounter) acquire(scope, id string, limit int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", scope, id)
	if m.count[key] >= int64(limit) {
		return false
	}
	m.count[key]++
	return true
}

func (m *memoryConcurrencyCounter) release(scope, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", scope, id)
	if m.count[key] > 0 {
		m.count[key]--
	}
}

// AcquireRelayConcurrency borrows concurrency slots for company/project/token
// scopes from the request context and returns a release function to be called
// via defer. Returns an error when any scope is over its limit.
func AcquireRelayConcurrency(c *gin.Context) (func(), error) {
	type slot struct {
		scope string
		id    string
		limit int
	}
	var slots []slot
	userId := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if company, err := LoadCompanyLimitsByOwner(c, userId); err == nil && company.MaxConcurrentRequests > 0 {
		slots = append(slots, slot{"company", strconv.Itoa(userId), company.MaxConcurrentRequests})
	}
	if rt, ok := c.Get(string(constant.ContextKeyBusinessProjectRuntime)); ok {
		if project, ok := rt.(*model.BusinessProjectRuntimePolicy); ok && project != nil && project.MaxConcurrentRequests > 0 {
			slots = append(slots, slot{"project", strconv.Itoa(project.ProjectId), project.MaxConcurrentRequests})
		}
	}
	if token, ok := c.Get(string(constant.ContextKeyToken)); ok {
		if t, ok := token.(*model.Token); ok && t != nil && t.MaxConcurrentRequests > 0 {
			slots = append(slots, slot{"token", strconv.Itoa(t.Id), t.MaxConcurrentRequests})
		}
	}
	acquired := make([]string, 0, len(slots))
	for _, s := range slots {
		ok, err := AcquireConcurrencySlot(c, s.scope, s.id, s.limit)
		if err != nil {
			for _, a := range acquired {
				parts := strings.SplitN(a, ":", 2)
				_ = ReleaseConcurrencySlot(c, parts[0], parts[1])
			}
			return nil, fmt.Errorf("concurrency acquire failed: %w", err)
		}
		if !ok {
			for _, a := range acquired {
				parts := strings.SplitN(a, ":", 2)
				_ = ReleaseConcurrencySlot(c, parts[0], parts[1])
			}
			return nil, errConcurrencyLimitExceeded
		}
		acquired = append(acquired, s.scope+":"+s.id)
	}
	return func() {
		for _, a := range acquired {
			parts := strings.SplitN(a, ":", 2)
			_ = ReleaseConcurrencySlot(c, parts[0], parts[1])
		}
	}, nil
}

var errConcurrencyLimitExceeded = errors.New("concurrency limit exceeded")
