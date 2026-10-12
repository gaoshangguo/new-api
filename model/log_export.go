package model

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// LogFilter is shared by paginated log queries and exports. UserID is always
// supplied by the authenticated controller, never bound from query parameters.
type LogFilter struct {
	UserID            *int   `form:"-"`
	Type              int    `form:"type" binding:"min=0,max=7"`
	StartTimestamp    int64  `form:"start_timestamp" binding:"min=0"`
	EndTimestamp      int64  `form:"end_timestamp" binding:"min=0"`
	ModelName         string `form:"model_name" binding:"max=1024"`
	Username          string `form:"username" binding:"max=1024"`
	TokenName         string `form:"token_name" binding:"max=1024"`
	Channel           int    `form:"channel" binding:"min=0"`
	Group             string `form:"group" binding:"max=1024"`
	RequestID         string `form:"request_id" binding:"max=64"`
	UpstreamRequestID string `form:"upstream_request_id" binding:"max=128"`
}

func (filter LogFilter) query(tx *gorm.DB) (*gorm.DB, error) {
	tx = tx.Model(&Log{})
	if filter.UserID != nil {
		tx = tx.Where("logs.user_id = ?", *filter.UserID)
	}
	if filter.Type != LogTypeUnknown {
		tx = tx.Where("logs.type = ?", filter.Type)
	}
	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", filter.ModelName); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", filter.Username); err != nil {
		return nil, err
	}
	if filter.TokenName != "" {
		tx = tx.Where("logs.token_name = ?", filter.TokenName)
	}
	if filter.RequestID != "" {
		tx = tx.Where("logs.request_id = ?", filter.RequestID)
	}
	if filter.UpstreamRequestID != "" {
		tx = tx.Where("logs.upstream_request_id = ?", filter.UpstreamRequestID)
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", filter.EndTimestamp)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", filter.Group)
	}
	return tx, nil
}

const MaxUsageLogExportRows = 10000
const maxUsageLogExportBytes = 20 * 1024 * 1024

var ErrUsageLogExportLimit = errors.New("too many logs to export; narrow the filters (maximum 10000 rows)")
var ErrUsageLogExportSize = errors.New("log export is too large; narrow the filters (maximum 20 MiB of text)")

// GetLogsForExport uses one bounded query, not offset pagination or a full count.
// Reading rows incrementally caps the aggregate text payload before allocating
// a workbook. A result exceeding either limit is rejected, never truncated.
func GetLogsForExport(ctx context.Context, filter LogFilter) ([]*Log, error) {
	tx, err := filter.query(LOG_DB.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	order := "logs.created_at desc, logs.id desc"
	if filter.UserID != nil {
		order = "logs.id desc"
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	rows, err := tx.Order(order).Limit(MaxUsageLogExportRows + 1).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]*Log, 0)
	textBytes := 0
	for rows.Next() {
		if len(logs) == MaxUsageLogExportRows {
			return nil, ErrUsageLogExportLimit
		}
		log := new(Log)
		if err := tx.ScanRows(rows, log); err != nil {
			return nil, err
		}
		for _, value := range []string{log.Content, log.Other, log.Username, log.TokenName, log.ModelName, log.Group, log.Ip, log.RequestId, log.UpstreamRequestId} {
			if len(value) > maxUsageLogExportBytes-textBytes {
				return nil, ErrUsageLogExportSize
			}
			textBytes += len(value)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if filter.UserID != nil {
		formatUserLogs(logs, 0)
	} else {
		if err := populateLogChannelNames(ctx, logs); err != nil {
			return nil, err
		}
	}
	return logs, nil
}
