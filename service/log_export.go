package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf16"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/xuri/excelize/v2"
)

// BuildUsageLogExcel writes values, never formulas, even for text beginning with
// '=' or '+'. Raw quota is exported to avoid rounding or display-currency loss.
// Cells too long for Excel are rejected instead of silently truncating logs.
func BuildUsageLogExcel(ctx context.Context, logs []*model.Log, admin bool) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Usage Logs"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return nil, err
	}
	headers := []interface{}{"Created at (UTC)", "Type", "Token name", "Model", "Group", "Quota (raw)",
		"Input tokens", "Output tokens", "Duration (s)", "Stream", "IP", "Request ID", "Upstream request ID", "Content", "Details (JSON)"}
	if admin {
		headers = append(headers, "User ID", "Username", "Channel ID", "Channel name")
	}
	costFactor, costUnit := 1.0, "quota"
	if operation_setting.IsCurrencyDisplay() {
		rate := operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
		if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return nil, fmt.Errorf("invalid quota display configuration")
		}
		costFactor, costUnit = rate/common.QuotaPerUnit, operation_setting.GetCurrencySymbol()
	}
	headers = append(headers, "Cost ("+costUnit+")")
	if err := sw.SetColWidth(1, len(headers), 22); err != nil {
		return nil, err
	}
	if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"305496"}}})
	if err != nil {
		return nil, err
	}
	if err := sw.SetRow("A1", headers, excelize.RowOpts{StyleID: style}); err != nil {
		return nil, err
	}
	for i, log := range logs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		logType := strconv.Itoa(log.Type)
		switch log.Type {
		case model.LogTypeTopup:
			logType = "Top-up"
		case model.LogTypeConsume:
			logType = "Consumption"
		case model.LogTypeManage:
			logType = "Management"
		case model.LogTypeSystem:
			logType = "System"
		case model.LogTypeError:
			logType = "Error"
		case model.LogTypeRefund:
			logType = "Refund"
		case model.LogTypeLogin:
			logType = "Login"
		}
		values := []interface{}{time.Unix(log.CreatedAt, 0).UTC().Format("2006-01-02 15:04:05"), logType,
			log.TokenName, log.ModelName, log.Group, log.Quota, log.PromptTokens, log.CompletionTokens,
			log.UseTime, log.IsStream, log.Ip, log.RequestId, log.UpstreamRequestId, log.Content, log.Other}
		if admin {
			values = append(values, log.UserId, log.Username, log.ChannelId, log.ChannelName)
		}
		cost := float64(log.Quota) * costFactor
		if math.IsNaN(cost) || math.IsInf(cost, 0) {
			return nil, fmt.Errorf("invalid displayed cost for log export")
		}
		values = append(values, cost)
		for col, value := range values {
			if str, ok := value.(string); ok {
				units := 0
				for _, r := range str {
					units += utf16.RuneLen(r)
				}
				if units > excelize.TotalCellChars {
					return nil, fmt.Errorf("log export cell exceeds Excel's text limit (row %d, column %d)", i+2, col+1)
				}
			}
		}
		if err := sw.SetRow(fmt.Sprintf("A%d", i+2), values); err != nil {
			return nil, err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
