package types

import "fmt"

type AccountingQuotaType string

const (
	AccountingQuotaTypeUnlimited AccountingQuotaType = "unlimited"
	AccountingQuotaTypeDaily     AccountingQuotaType = "daily"
	AccountingQuotaTypeMonthly   AccountingQuotaType = "monthly"
	AccountingQuotaTotal         AccountingQuotaType = "total"
	AccountingQuotaTypeScope     AccountingQuotaType = "scope"

	// per minute limit will use account_access_token_rates data
	AccountingQuotaTypePerMinute AccountingQuotaType = "per_minute"
)

type AccountingQuotaValueType string

const (
	AccountingQuotaValueTypeFee      AccountingQuotaValueType = "fee"
	AccountingQuotaValueTypeToken    AccountingQuotaValueType = "token"
	AccountingQuotaValueTypeRequest  AccountingQuotaValueType = "request"
	AccountingQuotaValueTypeModel    AccountingQuotaValueType = "models"   // scope or unlimited value
	AccountingQuotaValueTypePriority AccountingQuotaValueType = "priority" // scope value
)

type RequestPriority string

const (
	RequestPriorityHigh RequestPriority = "high"
	RequestPriorityLow  RequestPriority = "low"
)

func IsValidQuotaType(t AccountingQuotaType) bool {
	return t == AccountingQuotaTypeUnlimited ||
		t == AccountingQuotaTypeDaily ||
		t == AccountingQuotaTypeMonthly ||
		t == AccountingQuotaTotal ||
		t == AccountingQuotaTypeScope ||
		t == AccountingQuotaTypePerMinute
}

func IsValidQuotaValueType(v AccountingQuotaValueType) bool {
	return v == AccountingQuotaValueTypeFee ||
		v == AccountingQuotaValueTypeToken ||
		v == AccountingQuotaValueTypeRequest ||
		v == AccountingQuotaValueTypeModel ||
		v == AccountingQuotaValueTypePriority
}

func IsValidRequestPriority(p string) bool {
	return p == string(RequestPriorityHigh) || p == string(RequestPriorityLow) || p == ""
}

// maxQuotaRecordsPerValueType limits how many quota records a single value
// type may have within one quota set. Token limits allow two records so that
// a daily and a monthly token quota can coexist; every other value type
// allows exactly one record.
const maxQuotaRecordsPerValueType = 1
const maxQuotaRecordsPerValueTypeToken = 2

// isValidQuotaCombination reports whether a quota_type × value_type pair is
// actually enforced by the system. The quota middleware only checks
// daily/monthly/total quotas on fee/token usage and per_minute quotas on
// token/request usage; scope quotas only consume the models/priority value
// types. Combinations outside this matrix would be accepted silently but
// never enforced, so callers must reject them up front.
func isValidQuotaCombination(t AccountingQuotaType, v AccountingQuotaValueType) bool {
	switch t {
	case AccountingQuotaTypeUnlimited, AccountingQuotaTypeDaily, AccountingQuotaTypeMonthly, AccountingQuotaTotal:
		return v == AccountingQuotaValueTypeFee || v == AccountingQuotaValueTypeToken
	case AccountingQuotaTypePerMinute:
		return v == AccountingQuotaValueTypeToken || v == AccountingQuotaValueTypeRequest
	case AccountingQuotaTypeScope:
		return v == AccountingQuotaValueTypeModel || v == AccountingQuotaValueTypePriority
	}
	return false
}

// IsValidQuotaCombination is the exported form of isValidQuotaCombination for
// callers outside the types package.
func IsValidQuotaCombination(t AccountingQuotaType, v AccountingQuotaValueType) bool {
	return isValidQuotaCombination(t, v)
}

// CheckQuotaItems validates quota type/value combinations and set-level
// cardinality in one pass. Items are expected to carry valid enum values;
// use IsValidQuotaType and IsValidQuotaValueType for the enum check.
func CheckQuotaItems(quotaItems []UpdateAPIKeyQuotaItem, create bool) error {
	if len(quotaItems) == 0 {
		return nil
	}
	counts := make(map[AccountingQuotaValueType]int, len(quotaItems))
	for i, item := range quotaItems {
		if !isValidQuotaCombination(item.QuotaType, item.ValueType) {
			return fmt.Errorf("quotas[%d]: quota_type %s does not support value_type %s", i, item.QuotaType, item.ValueType)
		}
		counts[item.ValueType]++
	}
	if create {
		for valueType, count := range counts {
			maxRecords := maxQuotaRecordsPerValueType
			if valueType == AccountingQuotaValueTypeToken {
				maxRecords = maxQuotaRecordsPerValueTypeToken
			}
			if count > maxRecords {
				return fmt.Errorf("quota value type %s allows at most %d record(s), got %d", valueType, maxRecords, count)
			}
		}
	}
	return nil
}

func CheckQuotaSet(quotaItems []UpdateAPIKeyQuotaItem) error {
	if len(quotaItems) == 0 {
		return nil
	}
	counts := make(map[AccountingQuotaValueType]int, len(quotaItems))
	for _, item := range quotaItems {
		counts[item.ValueType]++
	}

	for valueType, count := range counts {
		maxRecords := maxQuotaRecordsPerValueType
		if valueType == AccountingQuotaValueTypeToken {
			maxRecords = maxQuotaRecordsPerValueTypeToken
		}
		if count > maxRecords {
			return fmt.Errorf("quota value type %s allows at most %d record(s), got %d", valueType, maxRecords, count)
		}
	}
	return nil
}
