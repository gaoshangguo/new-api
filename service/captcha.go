package service

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/google/uuid"
	"github.com/samber/hot"
)

// 图形验证码（自托管图片验证码）用于注册等匿名入口的人机校验，
// 与 Cloudflare Turnstile 相互独立，管理员可分别启用。
const (
	captchaCodeLength = 4
	// captchaCodeAlphabet 去除了 0/O、1/I/L 等易混淆字符，降低人工识别失败率。
	captchaCodeAlphabet   = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	captchaValidDuration  = 5 * time.Minute
	captchaCacheNamespace = "captcha:v1"
	captchaCacheCapacity  = 20000
)

// CaptchaChallenge 是下发给前端的一次性验证码挑战。
type CaptchaChallenge struct {
	CaptchaId string `json:"captcha_id"`
	// Image 为 data URL（image/png;base64），前端可直接用作 img 的 src。
	Image string `json:"image"`
}

var (
	captchaCacheOnce sync.Once
	captchaCache     *cachex.HybridCache[string]
)

func getCaptchaCache() *cachex.HybridCache[string] {
	captchaCacheOnce.Do(func() {
		captchaCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace: cachex.Namespace(captchaCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.StringCodec{},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, captchaCacheCapacity).
					WithTTL(captchaValidDuration).
					WithJanitor().
					Build()
			},
		})
	})
	return captchaCache
}

// NewCaptchaChallenge 生成一张图形验证码并暂存对应文本，返回给前端展示。
func NewCaptchaChallenge() (*CaptchaChallenge, error) {
	code, err := randomCaptchaCode(captchaCodeLength)
	if err != nil {
		return nil, err
	}
	image, err := renderCaptchaImage(code)
	if err != nil {
		return nil, err
	}
	captchaId := uuid.NewString()
	if err := getCaptchaCache().SetWithTTL(captchaId, code, captchaValidDuration); err != nil {
		return nil, err
	}
	return &CaptchaChallenge{
		CaptchaId: captchaId,
		Image:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(image),
	}, nil
}

// VerifyCaptcha 校验验证码并立即作废该挑战，避免同一张图片被反复猜测。
// 校验失败与挑战不存在都返回 false。
func VerifyCaptcha(captchaId string, code string) bool {
	captchaId = strings.TrimSpace(captchaId)
	code = strings.TrimSpace(code)
	if captchaId == "" || code == "" {
		return false
	}
	expected, found, err := getCaptchaCache().Get(captchaId)
	if err != nil {
		common.SysError("failed to read captcha challenge: " + err.Error())
		return false
	}
	if !found {
		return false
	}
	if _, err := getCaptchaCache().DeleteMany([]string{captchaId}); err != nil {
		common.SysError("failed to delete captcha challenge: " + err.Error())
	}
	return strings.EqualFold(expected, code)
}

func randomCaptchaCode(length int) (string, error) {
	alphabetSize := big.NewInt(int64(len(captchaCodeAlphabet)))
	code := make([]byte, length)
	for i := range code {
		index, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}
		code[i] = captchaCodeAlphabet[index.Int64()]
	}
	return string(code), nil
}
