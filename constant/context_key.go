package constant

type ContextKey string

const (
	ContextKeyTokenCountMeta  ContextKey = "token_count_meta"
	ContextKeyPromptTokens    ContextKey = "prompt_tokens"
	ContextKeyEstimatedTokens ContextKey = "estimated_tokens"

	ContextKeyOriginalModel    ContextKey = "original_model"
	ContextKeyRequestStartTime ContextKey = "request_start_time"

	/* token related keys */
	ContextKeyTokenUnlimited         ContextKey = "token_unlimited_quota"
	ContextKeyTokenKey               ContextKey = "token_key"
	ContextKeyTokenId                ContextKey = "token_id"
	ContextKeyTokenGroup             ContextKey = "token_group"
	ContextKeyTokenSpecificChannelId ContextKey = "specific_channel_id"
	ContextKeyTokenModelLimitEnabled ContextKey = "token_model_limit_enabled"
	ContextKeyTokenModelLimit        ContextKey = "token_model_limit"
	ContextKeyTokenChannelLimits     ContextKey = "token_channel_limits"
	ContextKeyTokenCrossGroupRetry   ContextKey = "token_cross_group_retry"
	// ContextKeyToken stores the authenticated *model.Token so downstream
	// request-scoped guards (e.g. layered RPM/TPM rate limits) can read the
	// token's limit fields without another cache or DB lookup.
	ContextKeyToken ContextKey = "token"
	// ContextKeyBusinessProjectRuntime stores the authoritative runtime policy
	// for a token bound to an enterprise project. It is set during TokenAuth
	// and used again by model/channel selection and billing guards.
	ContextKeyBusinessProjectRuntime ContextKey = "business_project_runtime"

	/* channel related keys */
	ContextKeyChannelId                ContextKey = "channel_id"
	ContextKeyChannelName              ContextKey = "channel_name"
	ContextKeyChannelCreateTime        ContextKey = "channel_create_time"
	ContextKeyChannelBaseUrl           ContextKey = "base_url"
	ContextKeyChannelType              ContextKey = "channel_type"
	ContextKeyChannelSetting           ContextKey = "channel_setting"
	ContextKeyChannelOtherSetting      ContextKey = "channel_other_setting"
	ContextKeyChannelParamOverride     ContextKey = "param_override"
	ContextKeyChannelHeaderOverride    ContextKey = "header_override"
	ContextKeyChannelOrganization      ContextKey = "channel_organization"
	ContextKeyChannelAutoBan           ContextKey = "auto_ban"
	ContextKeyChannelModelMapping      ContextKey = "model_mapping"
	ContextKeyChannelStatusCodeMapping ContextKey = "status_code_mapping"
	ContextKeyChannelIsMultiKey        ContextKey = "channel_is_multi_key"
	ContextKeyChannelMultiKeyIndex     ContextKey = "channel_multi_key_index"
	ContextKeyChannelKey               ContextKey = "channel_key"

	ContextKeyAutoGroup           ContextKey = "auto_group"
	ContextKeyAutoGroupIndex      ContextKey = "auto_group_index"
	ContextKeyAutoGroupRetryIndex ContextKey = "auto_group_retry_index"

	/* user related keys */
	ContextKeyUserId      ContextKey = "id"
	ContextKeyUserSetting ContextKey = "user_setting"
	ContextKeyUserQuota   ContextKey = "user_quota"
	ContextKeyUserStatus  ContextKey = "user_status"
	ContextKeyUserEmail   ContextKey = "user_email"
	ContextKeyUserGroup   ContextKey = "user_group"
	ContextKeyUsingGroup  ContextKey = "group"
	ContextKeyUserName    ContextKey = "username"

	ContextKeyLocalCountTokens ContextKey = "local_count_tokens"

	ContextKeySystemPromptOverride ContextKey = "system_prompt_override"

	// ContextKeyFileSourcesToCleanup stores file sources that need cleanup when request ends
	ContextKeyFileSourcesToCleanup ContextKey = "file_sources_to_cleanup"

	// ContextKeyAdminRejectReason stores an admin-only reject/block reason extracted from upstream responses.
	// It is not returned to end users, but can be persisted into consume/error logs for debugging.
	ContextKeyAdminRejectReason ContextKey = "admin_reject_reason"

	// ContextKeyLanguage stores the user's language preference for i18n
	ContextKeyLanguage ContextKey = "language"
	ContextKeyIsStream ContextKey = "is_stream"

	// ContextKeyAuditLogged marks that the current request has already recorded
	// a manage/operation audit log inside the handler. When set, the admin-audit
	// fallback in authHelper (finishAdminAudit) skips its record to avoid
	// duplicate entries.
	ContextKeyAuditLogged ContextKey = "audit_logged"

	// ContextKeyModelAlias / ContextKeyModelAliasVersion record that the
	// request model was resolved through a model alias (P0-10), for log
	// explainability and channel allowlist enforcement.
	ContextKeyModelAlias        ContextKey = "model_alias"
	ContextKeyModelAliasVersion ContextKey = "model_alias_version"
	ContextKeyModelAliasChannelIds ContextKey = "model_alias_channel_ids"

	// ContextKeyRequestRegion is the request-specified region (X-API-Region
	// header) used by the P0-14 multi-dimension channel routing.
	ContextKeyRequestRegion ContextKey = "request_region"

	// ContextKeyRoutingTrace holds the structured per-request routing trace
	// (P0-15): candidates, per-attempt channel + failure reason, final channel.
	ContextKeyRoutingTrace ContextKey = "routing_trace"
)
