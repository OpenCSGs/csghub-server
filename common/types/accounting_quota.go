package types

type AccountingQuotaType string

const (
	AccountingQuotaTypeUnlimited AccountingQuotaType = "unlimited"
	AccountingQuotaTypeMonthly   AccountingQuotaType = "monthly"
	AccountingQuotaTotal         AccountingQuotaType = "total"
)

type AccountingQuotaValueType string

const (
	AccountingQuotaValueTypeFee AccountingQuotaValueType = "fee"
)
