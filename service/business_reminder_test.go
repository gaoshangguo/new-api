package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueBusinessReminderScanUsesDeduplicatedSystemTask(t *testing.T) {
	truncate(t)

	first, created, err := EnqueueBusinessReminderScan()
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, first)
	assert.Equal(t, model.SystemTaskTypeBusinessReminderScan, first.Type)

	existing, created, err := EnqueueBusinessReminderScan()
	require.NoError(t, err)
	assert.False(t, created)
	require.NotNil(t, existing)
	assert.Equal(t, first.TaskID, existing.TaskID)
}

func TestBusinessReminderTaskIntervalIsConservative(t *testing.T) {
	assert.GreaterOrEqual(t, businessReminderTaskHandler{}.Interval(), time.Hour)
}
