package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

const scopeRateLimitWindowSeconds = 60

type ScopeRateLimit struct {
	RPM int   // 0 = unlimited
	TPM int64 // 0 = unlimited
}

// memoryRateLimitWindow is the Redis-unavailable fallback: per-key counters
// with a fixed 60s window, single-node semantics.
type memoryRateLimitWindow struct {
	mu      sync.Mutex
	counter map[string]int64
	start   map[string]int64
}

var memoryRateLimit = &memoryRateLimitWindow{counter: map[string]int64{}, start: map[string]int64{}}

// CheckScopeRateLimit enforces RPM (exact request count, INCR +1) and TPM
// (estimated tokens, INCRBY amount) over a 60-second fixed window. Returns
// false when the request is over the limit. Redis Lua-atomic counters with
// in-memory fallback when Redis is unavailable (single-node semantics).
func CheckScopeRateLimit(ctx context.Context, scope string, id string, cfg ScopeRateLimit, estimatedTokens int64) (bool, error) {
	if cfg.RPM <= 0 && cfg.TPM <= 0 {
		return true, nil
	}
	rdb := common.RDB
	if rdb == nil {
		return checkScopeRateLimitMemory(scope, id, cfg, estimatedTokens), nil
	}
	now := time.Now().Unix()
	window := now / scopeRateLimitWindowSeconds
	if cfg.RPM > 0 {
		key := fmt.Sprintf("rl:rpm:%s:%s:%d", scope, id, window)
		allowed, err := scopeWindowAllow(ctx, rdb, key, int64(cfg.RPM))
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	if cfg.TPM > 0 && estimatedTokens > 0 {
		key := fmt.Sprintf("rl:tpm:%s:%s:%d", scope, id, window)
		amount := estimatedTokens
		if amount > cfg.TPM {
			amount = cfg.TPM // 单请求封顶，避免负值语义
		}
		allowed, err := scopeWindowAdd(ctx, rdb, key, amount, cfg.TPM)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}

// scopeWindowAllow is a Lua-atomic INCR-based check for request counts: when
// the counter is already at the limit the increment is rolled back and 0 is
// returned. Keys expire after 120s (two windows) to avoid stale growth.
var scopeWindowAllowScript = redis.NewScript(`
local v = redis.call('INCR', KEYS[1])
if v > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
redis.call('EXPIRE', KEYS[1], 120)
return 1
`)

func scopeWindowAllow(ctx context.Context, rdb *redis.Client, key string, limit int64) (bool, error) {
	res, err := scopeWindowAllowScript.Run(ctx, rdb, []string{key}, limit).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// scopeWindowAdd is a Lua-atomic INCRBY-based check for token estimates: the
// counter is increased by amount (capped at limit) unless the window is full.
var scopeWindowAddScript = redis.NewScript(`
local v = redis.call('INCRBY', KEYS[1], tonumber(ARGV[1]))
if v > tonumber(ARGV[2]) then
  redis.call('DECRBY', KEYS[1], tonumber(ARGV[1]))
  return 0
end
redis.call('EXPIRE', KEYS[1], 120)
return 1
`)

func scopeWindowAdd(ctx context.Context, rdb *redis.Client, key string, amount, limit int64) (bool, error) {
	res, err := scopeWindowAddScript.Run(ctx, rdb, []string{key}, amount, limit).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// checkScopeRateLimitMemory is the Redis-unavailable fallback: per-key 60s
// window counters guarded by a mutex, single-node semantics.
func checkScopeRateLimitMemory(scope string, id string, cfg ScopeRateLimit, estimatedTokens int64) bool {
	memoryRateLimit.mu.Lock()
	defer memoryRateLimit.mu.Unlock()
	now := time.Now().Unix()
	window := now / scopeRateLimitWindowSeconds
	if cfg.RPM > 0 {
		key := fmt.Sprintf("rpm:%s:%s", scope, id)
		memoryWindowReset(key, window)
		if memoryRateLimit.counter[key] >= int64(cfg.RPM) {
			return false
		}
		memoryRateLimit.counter[key]++
	}
	if cfg.TPM > 0 && estimatedTokens > 0 {
		key := fmt.Sprintf("tpm:%s:%s", scope, id)
		memoryWindowReset(key, window)
		if memoryRateLimit.counter[key] >= cfg.TPM {
			return false
		}
		memoryRateLimit.counter[key] += estimatedTokens
	}
	return true
}

func memoryWindowReset(key string, window int64) {
	if memoryRateLimit.start[key] != window {
		memoryRateLimit.start[key] = window
		memoryRateLimit.counter[key] = 0
	}
}

// EnforceRelayScopeRateLimits checks enterprise -> project -> token -> model
// rate limits after authentication. estimatedTokens is the request's estimated
// input tokens (0 = skip TPM checks). Returns an error when over the limit.
func EnforceRelayScopeRateLimits(c *gin.Context, modelName string, estimatedTokens int64) error {
	userId := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	// 企业级
	companyLimit, err := LoadCompanyLimitsByOwner(c, userId)
	if err != nil {
		common.SysError(fmt.Sprintf("company rate limit lookup failed for user %d: %v", userId, err))
		companyLimit = ScopeRateLimit{}
	}
	if companyLimit.RPM > 0 || companyLimit.TPM > 0 {
		ok, err := CheckScopeRateLimit(c, "company", strconv.Itoa(userId), companyLimit, estimatedTokens)
		if err != nil {
			return fmt.Errorf("company rate limit check failed: %w", err)
		}
		if !ok {
			return errScopeRateLimitExceeded
		}
	}
	// 项目级
	if rt, ok := c.Get(string(constant.ContextKeyBusinessProjectRuntime)); ok {
		if project, ok := rt.(*model.BusinessProjectRuntimePolicy); ok && project != nil {
			cfg := ScopeRateLimit{RPM: project.RateLimitRPM, TPM: project.RateLimitTPM}
			ok, err := CheckScopeRateLimit(c, "project", strconv.Itoa(project.ProjectId), cfg, estimatedTokens)
			if err != nil {
				return fmt.Errorf("project rate limit check failed: %w", err)
			}
			if !ok {
				return errScopeRateLimitExceeded
			}
		}
	}
	// Key 级
	if token, ok := c.Get(string(constant.ContextKeyToken)); ok {
		if t, ok := token.(*model.Token); ok && t != nil {
			cfg := ScopeRateLimit{RPM: t.RateLimitRPM, TPM: t.RateLimitTPM}
			ok, err := CheckScopeRateLimit(c, "token", strconv.Itoa(t.Id), cfg, estimatedTokens)
			if err != nil {
				return fmt.Errorf("token rate limit check failed: %w", err)
			}
			if !ok {
				return errScopeRateLimitExceeded
			}
		}
	}
	// 模型级（全局 env option）
	modelCfg := ScopeRateLimit{RPM: common.ModelRateLimitRPM, TPM: common.ModelRateLimitTPM}
	if modelCfg.RPM > 0 || modelCfg.TPM > 0 {
		ok, err := CheckScopeRateLimit(c, "model", modelName, modelCfg, estimatedTokens)
		if err != nil {
			return fmt.Errorf("model rate limit check failed: %w", err)
		}
		if !ok {
			return errScopeRateLimitExceeded
		}
	}
	return nil
}

// EnforceChannelRateLimit checks the selected channel's rate limits.
func EnforceChannelRateLimit(c *gin.Context, channelID int, channelRPM int, channelTPM int64, estimatedTokens int64) error {
	if channelRPM <= 0 && channelTPM <= 0 {
		return nil
	}
	cfg := ScopeRateLimit{RPM: channelRPM, TPM: channelTPM}
	ok, err := CheckScopeRateLimit(c, "channel", strconv.Itoa(channelID), cfg, estimatedTokens)
	if err != nil {
		return fmt.Errorf("channel rate limit check failed: %w", err)
	}
	if !ok {
		return errScopeRateLimitExceeded
	}
	return nil
}

var errScopeRateLimitExceeded = errors.New("rate limit exceeded")

// companyLimitsCacheEntry is a short-lived snapshot of a company's rate limit
// configuration, keyed by owner user ID.
type companyLimitsCacheEntry struct {
	cfg       ScopeRateLimit
	expiresAt int64
}

// companyLimitsCacheStore is a simple map + mutex cache with a 5-minute TTL.
type companyLimitsCacheStore struct {
	mu      sync.Mutex
	entries map[int]companyLimitsCacheEntry
}

const companyLimitsCacheTTLSeconds = 5 * 60

func (c *companyLimitsCacheStore) Get(ownerUserID int) (ScopeRateLimit, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[ownerUserID]
	if !ok {
		return ScopeRateLimit{}, false
	}
	if time.Now().Unix() >= entry.expiresAt {
		delete(c.entries, ownerUserID)
		return ScopeRateLimit{}, false
	}
	return entry.cfg, true
}

func (c *companyLimitsCacheStore) Set(ownerUserID int, cfg ScopeRateLimit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[ownerUserID] = companyLimitsCacheEntry{
		cfg:       cfg,
		expiresAt: time.Now().Unix() + companyLimitsCacheTTLSeconds,
	}
}

var companyLimitsCache = &companyLimitsCacheStore{entries: map[int]companyLimitsCacheEntry{}}

// LoadCompanyLimitsByOwner resolves the owner's company rate limits with a
// short in-memory cache.
func LoadCompanyLimitsByOwner(ctx context.Context, ownerUserID int) (ScopeRateLimit, error) {
	cached, ok := companyLimitsCache.Get(ownerUserID)
	if ok {
		return cached, nil
	}
	var company model.Company
	err := model.DB.Where("owner_user_id = ?", ownerUserID).First(&company).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			companyLimitsCache.Set(ownerUserID, ScopeRateLimit{})
			return ScopeRateLimit{}, nil
		}
		return ScopeRateLimit{}, err
	}
	cfg := ScopeRateLimit{RPM: company.RateLimitRPM, TPM: company.RateLimitTPM}
	companyLimitsCache.Set(ownerUserID, cfg)
	return cfg, nil
}
