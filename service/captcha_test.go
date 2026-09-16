package service

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const captchaDataURLPrefix = "data:image/png;base64,"

func storeCaptchaChallenge(t *testing.T, captchaId string, code string) {
	t.Helper()
	require.NoError(t, getCaptchaCache().SetWithTTL(captchaId, code, captchaValidDuration))
}

func TestNewCaptchaChallengeReturnsPNGDataURL(t *testing.T) {
	challenge, err := NewCaptchaChallenge()
	require.NoError(t, err)
	require.NotEmpty(t, challenge.CaptchaId)
	require.True(t, strings.HasPrefix(challenge.Image, captchaDataURLPrefix))

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(challenge.Image, captchaDataURLPrefix))
	require.NoError(t, err)
	_, err = png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
}

func TestVerifyCaptchaAcceptsCodeIgnoringCase(t *testing.T) {
	storeCaptchaChallenge(t, "captcha-case", "A2B3")

	assert.True(t, VerifyCaptcha("captcha-case", "a2b3"))
}

func TestVerifyCaptchaConsumesChallengeAfterSuccess(t *testing.T) {
	storeCaptchaChallenge(t, "captcha-single-use", "C4D5")

	assert.True(t, VerifyCaptcha("captcha-single-use", "C4D5"))
	assert.False(t, VerifyCaptcha("captcha-single-use", "C4D5"))
}

func TestVerifyCaptchaConsumesChallengeAfterFailure(t *testing.T) {
	storeCaptchaChallenge(t, "captcha-wrong-code", "E6F7")

	assert.False(t, VerifyCaptcha("captcha-wrong-code", "ZZZZ"))
	assert.False(t, VerifyCaptcha("captcha-wrong-code", "E6F7"))
}

func TestVerifyCaptchaRejectsEmptyOrUnknownChallenge(t *testing.T) {
	assert.False(t, VerifyCaptcha("", "A2B3"))
	assert.False(t, VerifyCaptcha("captcha-unknown", "A2B3"))
	assert.False(t, VerifyCaptcha("captcha-unknown", ""))
}
