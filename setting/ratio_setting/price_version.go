package ratio_setting

// Price snapshot support for price versioning (P0-28). A price version is a
// frozen snapshot of every price-affecting map at a point in time; the keys
// here are the option keys that participate in a snapshot.
var priceSnapshotKeys = []string{
	"ModelRatio",
	"ModelPrice",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
	"GroupRatio",
	"GroupGroupRatio",
}

// IsPriceSnapshotKey reports whether the option key participates in a price
// version snapshot. Only these keys are accepted as version overlays and are
// captured when a version is created.
func IsPriceSnapshotKey(key string) bool {
	for _, k := range priceSnapshotKeys {
		if k == key {
			return true
		}
	}
	return false
}

// PriceSnapshotKeys returns the option keys that participate in a price
// version snapshot (ordered, stable).
func PriceSnapshotKeys() []string {
	keys := make([]string, len(priceSnapshotKeys))
	copy(keys, priceSnapshotKeys)
	return keys
}

// GetPriceMapsJSON snapshots the current live price maps as JSON strings keyed
// by option key. Values are the exact persisted option payloads, so a snapshot
// can be restored later through the same updateOptionMap dispatch.
func GetPriceMapsJSON() map[string]string {
	return map[string]string{
		"ModelRatio":           ModelRatio2JSONString(),
		"ModelPrice":           ModelPrice2JSONString(),
		"CompletionRatio":      CompletionRatio2JSONString(),
		"CacheRatio":           CacheRatio2JSONString(),
		"CreateCacheRatio":     CreateCacheRatio2JSONString(),
		"ImageRatio":           ImageRatio2JSONString(),
		"AudioRatio":           AudioRatio2JSONString(),
		"AudioCompletionRatio": AudioCompletionRatio2JSONString(),
		"GroupRatio":           GroupRatio2JSONString(),
		"GroupGroupRatio":      GroupGroupRatio2JSONString(),
	}
}
