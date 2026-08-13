package dto

type Notify struct {
	Type    string        `json:"type"`
	Title   string        `json:"title"`
	Content string        `json:"content"`
	Values  []interface{} `json:"values"`
}

const ContentValueParam = "{{value}}"

const (
	NotifyTypeQuotaExceed      = "quota_exceed"
	NotifyTypeChannelUpdate    = "channel_update"
	NotifyTypeChannelTest      = "channel_test"
	NotifyTypeBusinessReminder = "business_reminder"
	// NotifyTypeSecurityAlert 登录安全提醒（新设备/新地点登录，P0-03）。
	NotifyTypeSecurityAlert = "security_alert"
	// NotifyTypePlatformAlert 平台级告警（P0-29）。
	NotifyTypePlatformAlert = "platform_alert"
)

func NewNotify(t string, title string, content string, values []interface{}) Notify {
	return Notify{
		Type:    t,
		Title:   title,
		Content: content,
		Values:  values,
	}
}
