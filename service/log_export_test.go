package service

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func setupUsageLogExcelDisplay(t *testing.T) {
	t.Helper()
	previous := *operation_setting.GetGeneralSetting()
	previousQuota, previousRate := common.QuotaPerUnit, operation_setting.USDExchangeRate
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	common.QuotaPerUnit, operation_setting.USDExchangeRate = 500000, 7.3
	t.Cleanup(func() {
		*operation_setting.GetGeneralSetting() = previous
		common.QuotaPerUnit, operation_setting.USDExchangeRate = previousQuota, previousRate
	})
}

func TestUsageLogExcelPreservesNumbersAndTreatsUntrustedTextAsLiteral(t *testing.T) {
	setupUsageLogExcelDisplay(t)
	log := &model.Log{CreatedAt: 100, Type: model.LogTypeConsume, TokenName: "=HYPERLINK(\"https://example.invalid\")", ModelName: "+SUM(1,2)", Group: "@unsafe", Quota: 125000, PromptTokens: 12, CompletionTokens: 34, UseTime: 2, IsStream: true, RequestId: "0001234567890123456789", Content: "中文 <>&\nsecond line", Other: `{"model_price":0.25}`, UserId: 1, Username: "alice", ChannelId: 3, ChannelName: "primary"}
	data, err := BuildUsageLogExcel(context.Background(), []*model.Log{log}, true)
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	rows, err := f.GetRows("Usage Logs")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"1970-01-01 00:01:40", "Consumption", log.TokenName, log.ModelName, log.Group, "125000", "12", "34", "2", "TRUE", "", log.RequestId, "", log.Content, log.Other, "1", "alice", "3", "primary", "0.25"}, rows[1])
	for _, cell := range []string{"C2", "D2", "E2", "L2"} {
		formula, err := f.GetCellFormula("Usage Logs", cell)
		require.NoError(t, err)
		assert.Empty(t, formula, "untrusted data must never create an Excel formula")
		cellType, err := f.GetCellType("Usage Logs", cell)
		require.NoError(t, err)
		assert.Equal(t, excelize.CellTypeInlineString, cellType)
	}
	require.NoError(t, f.SetCellFormula("Usage Logs", "F4", "SUM(F2:F2)"))
	sum, err := f.CalcCellValue("Usage Logs", "F4")
	require.NoError(t, err)
	assert.Equal(t, "125000", sum, "quota must be numeric and usable in Excel calculations")
}

func TestUsageLogExcelSelfExportOmitsAdministratorColumns(t *testing.T) {
	data, err := BuildUsageLogExcel(context.Background(), []*model.Log{{Username: "admin-name", ChannelId: 1, ChannelName: "hidden"}}, false)
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()
	rows, err := f.GetRows("Usage Logs")
	require.NoError(t, err)
	assert.Len(t, rows[0], 16)
	assert.NotContains(t, rows[0], "Username")
	assert.NotContains(t, rows[0], "Channel ID")
	assert.NotContains(t, rows[1], "admin-name")
	assert.NotContains(t, rows[1], "hidden")
}

func TestUsageLogExcelCostUsesConfiguredDisplayUnits(t *testing.T) {
	setupUsageLogExcelDisplay(t)
	cases := []struct{ displayType, header, cost string }{
		{operation_setting.QuotaDisplayTypeUSD, "Cost ($)", "1"},
		{operation_setting.QuotaDisplayTypeCNY, "Cost (¥)", "7.3"},
		{operation_setting.QuotaDisplayTypeCustom, "Cost (积分)", "20"},
		{operation_setting.QuotaDisplayTypeTokens, "Cost (quota)", "500000"},
	}
	for _, tc := range cases {
		t.Run(tc.displayType, func(t *testing.T) {
			setting := operation_setting.GetGeneralSetting()
			setting.QuotaDisplayType, setting.CustomCurrencySymbol, setting.CustomCurrencyExchangeRate = tc.displayType, "积分", 20
			data, err := BuildUsageLogExcel(context.Background(), []*model.Log{{Quota: 500000}}, false)
			require.NoError(t, err)
			f, err := excelize.OpenReader(bytes.NewReader(data))
			require.NoError(t, err)
			defer f.Close()
			rows, err := f.GetRows("Usage Logs")
			require.NoError(t, err)
			assert.Equal(t, tc.header, rows[0][15])
			assert.Equal(t, tc.cost, rows[1][15])
		})
	}
}

func TestUsageLogExcelEmptyResultStillHasColumnHeaders(t *testing.T) {
	data, err := BuildUsageLogExcel(context.Background(), nil, false)
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()
	rows, err := f.GetRows("Usage Logs")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Created at (UTC)", rows[0][0])
}

func TestUsageLogExcelRejectsTextBeyondExcelLimitWithoutTruncation(t *testing.T) {
	for _, text := range []string{strings.Repeat("a", 32768), strings.Repeat("😀", 16384)} {
		data, err := BuildUsageLogExcel(context.Background(), []*model.Log{{Content: text}}, false)
		require.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "text limit")
	}
}

func TestUsageLogExcelCancelledRequestProducesNoDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data, err := BuildUsageLogExcel(ctx, []*model.Log{{Content: "cancelled"}}, false)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, data)
}
