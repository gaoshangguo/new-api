package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

type channelKeyReEncryptHandler struct{}

func init() {
	RegisterSystemTaskHandler(channelKeyReEncryptHandler{})
}

func (channelKeyReEncryptHandler) Type() string { return model.SystemTaskTypeChannelKeyReEncrypt }

func (channelKeyReEncryptHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := ReEncryptAllChannelKeys(ctx)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("channel key re-encrypt task %s failed to persist result: %v", task.TaskID, err))
	}
}

// EnqueueChannelKeyReEncrypt queues a manual re-encryption task through the
// same DB-leased SystemTask path as scheduled runs.
func EnqueueChannelKeyReEncrypt() (*model.SystemTask, bool, error) {
	return EnqueueSystemTask(model.SystemTaskTypeChannelKeyReEncrypt, nil)
}

type ChannelKeyReEncryptResult struct {
	Total       int `json:"total"`
	ReEncrypted int `json:"re_encrypted"`
	Failed      int `json:"failed"`
}

// ReEncryptAllChannelKeys rewrites every channel key with the current master
// key. Decryption tries the current key then the optional previous key
// (CHANNEL_KEY_MASTER_KEY_PREVIOUS), so this can run right after switching to
// a new master key without data loss.
func ReEncryptAllChannelKeys(_ context.Context) (ChannelKeyReEncryptResult, error) {
	// GetAllChannels(selectAll=true) returns every channel including its key in
	// a single query (offset/num are ignored in that mode).
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return ChannelKeyReEncryptResult{}, err
	}
	result := ChannelKeyReEncryptResult{}
	for _, channel := range channels {
		result.Total++
		if channel.Key == "" {
			continue
		}
		plain, err := common.DecryptChannelKey(channel.Key)
		if err != nil {
			result.Failed++
			common.SysError(fmt.Sprintf("channel %d key re-encrypt decrypt failed: %v", channel.Id, err))
			continue
		}
		encrypted, err := common.EncryptChannelKey(plain)
		if err != nil {
			result.Failed++
			common.SysError(fmt.Sprintf("channel %d key re-encrypt encrypt failed: %v", channel.Id, err))
			continue
		}
		channel.Key = encrypted
		if err := channel.Save(); err != nil {
			result.Failed++
			common.SysError(fmt.Sprintf("channel %d key re-encrypt save failed: %v", channel.Id, err))
			continue
		}
		result.ReEncrypted++
	}
	return result, nil
}
