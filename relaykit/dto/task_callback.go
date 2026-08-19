package dto

// TaskCallbackPayload is delivered to a customer-provided callback_url when an
// async task (video generation, Midjourney, etc.) reaches a terminal state.
// The body is signed with HMAC-SHA256 over the exact JSON payload; the hex
// digest is sent in the X-New-Api-Signature header so customers can verify
// that the notification really came from the gateway.
type TaskCallbackPayload struct {
	// Type identifies the task family: "task" (video/audio/etc.) or
	// "midjourney".
	Type string `json:"type"`
	// TaskId is the public task identifier (task_xxx for generic tasks, the
	// mj id for Midjourney).
	TaskId string `json:"task_id"`
	// Status is the terminal status: SUCCESS or FAILURE.
	Status string `json:"status"`
	// ModelName is the model the task was submitted with.
	ModelName string `json:"model_name"`
	// Quota is the final charged quota after settlement/refund.
	Quota int `json:"quota"`
	// Error is the failure reason when Status == FAILURE.
	Error string `json:"error,omitempty"`
	// ResultUrl is the downloadable result URL when available.
	ResultUrl string `json:"result_url,omitempty"`
	// Timestamp is the unix second at delivery time.
	Timestamp int64 `json:"timestamp"`
}
