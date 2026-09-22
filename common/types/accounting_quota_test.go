package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckQuotaSet(t *testing.T) {
	t.Run("empty quota set is valid", func(t *testing.T) {
		assert.NoError(t, CheckQuotaSet(nil))
		assert.NoError(t, CheckQuotaSet([]UpdateAPIKeyQuotaItem{}))
	})

	t.Run("one record per value type is valid", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeToken, Quota: 50},
			{QuotaType: AccountingQuotaTotal, ValueType: AccountingQuotaValueTypeRequest, Quota: 1000},
			{QuotaType: AccountingQuotaTypeScope, ValueType: AccountingQuotaValueTypeModel},
			{QuotaType: AccountingQuotaTypeScope, ValueType: AccountingQuotaValueTypePriority},
		}
		assert.NoError(t, CheckQuotaSet(items))
	})

	t.Run("two token records are valid", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeToken, Quota: 50},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeToken, Quota: 500},
		}
		assert.NoError(t, CheckQuotaSet(items))
	})

	t.Run("three token records are invalid", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeToken, Quota: 50},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeToken, Quota: 500},
			{QuotaType: AccountingQuotaTotal, ValueType: AccountingQuotaValueTypeToken, Quota: 5000},
		}
		err := CheckQuotaSet(items)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), string(AccountingQuotaValueTypeToken))
	})

	t.Run("duplicate records for a non-token value type are invalid", func(t *testing.T) {
		for _, valueType := range []AccountingQuotaValueType{
			AccountingQuotaValueTypeFee,
			AccountingQuotaValueTypeRequest,
			AccountingQuotaValueTypeModel,
			AccountingQuotaValueTypePriority,
		} {
			items := []UpdateAPIKeyQuotaItem{
				{QuotaType: AccountingQuotaTypeDaily, ValueType: valueType, Quota: 50},
				{QuotaType: AccountingQuotaTypeMonthly, ValueType: valueType, Quota: 500},
			}
			err := CheckQuotaSet(items)
			assert.Error(t, err, "value type %s should reject duplicates", valueType)
			assert.Contains(t, err.Error(), string(valueType))
		}
	})

	t.Run("mixed valid token records with duplicate fee records are invalid", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeToken, Quota: 50},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeToken, Quota: 500},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeFee, Quota: 10},
		}
		err := CheckQuotaSet(items)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), string(AccountingQuotaValueTypeFee))
	})
}

func TestIsValidQuotaCombination(t *testing.T) {
	valid := []struct {
		quotaType AccountingQuotaType
		valueType AccountingQuotaValueType
	}{
		{AccountingQuotaTypeUnlimited, AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeUnlimited, AccountingQuotaValueTypeToken},
		{AccountingQuotaTypeDaily, AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeDaily, AccountingQuotaValueTypeToken},
		{AccountingQuotaTypeMonthly, AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeMonthly, AccountingQuotaValueTypeToken},
		{AccountingQuotaTotal, AccountingQuotaValueTypeFee},
		{AccountingQuotaTotal, AccountingQuotaValueTypeToken},
		{AccountingQuotaTypePerMinute, AccountingQuotaValueTypeToken},
		{AccountingQuotaTypePerMinute, AccountingQuotaValueTypeRequest},
		{AccountingQuotaTypeScope, AccountingQuotaValueTypeModel},
		{AccountingQuotaTypeScope, AccountingQuotaValueTypePriority},
	}
	for _, c := range valid {
		assert.True(t, IsValidQuotaCombination(c.quotaType, c.valueType),
			"%s+%s should be a valid combination", c.quotaType, c.valueType)
	}

	// Combinations the system accepts but never enforces must be invalid.
	invalid := []struct {
		quotaType AccountingQuotaType
		valueType AccountingQuotaValueType
	}{
		{AccountingQuotaTypeMonthly, AccountingQuotaValueTypeRequest},
		{AccountingQuotaTypeDaily, AccountingQuotaValueTypeRequest},
		{AccountingQuotaTotal, AccountingQuotaValueTypeRequest},
		{AccountingQuotaTypePerMinute, AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeScope, AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeScope, AccountingQuotaValueTypeToken},
		{AccountingQuotaTypeUnlimited, AccountingQuotaValueTypeModel},
		{AccountingQuotaTypeUnlimited, AccountingQuotaValueTypePriority},
		{AccountingQuotaTypeUnlimited, AccountingQuotaValueTypeRequest},
		{AccountingQuotaType("bogus"), AccountingQuotaValueTypeFee},
		{AccountingQuotaTypeMonthly, AccountingQuotaValueType("bogus")},
	}
	for _, c := range invalid {
		assert.False(t, IsValidQuotaCombination(c.quotaType, c.valueType),
			"%s+%s should be an invalid combination", c.quotaType, c.valueType)
	}
}

func TestCheckQuotaItems(t *testing.T) {
	t.Run("empty quota set is valid", func(t *testing.T) {
		assert.NoError(t, CheckQuotaItems(nil, true))
		assert.NoError(t, CheckQuotaItems([]UpdateAPIKeyQuotaItem{}, false))
	})

	t.Run("enforceable combinations pass", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeToken, Quota: 50},
			{QuotaType: AccountingQuotaTypePerMinute, ValueType: AccountingQuotaValueTypeRequest, Quota: 60},
			{QuotaType: AccountingQuotaTypeScope, ValueType: AccountingQuotaValueTypeModel},
			{QuotaType: AccountingQuotaTypeScope, ValueType: AccountingQuotaValueTypePriority},
		}
		assert.NoError(t, CheckQuotaItems(items, true))
		assert.NoError(t, CheckQuotaItems(items, false))
	})

	t.Run("unenforceable combination is rejected with index", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTotal, ValueType: AccountingQuotaValueTypeRequest, Quota: 1000},
		}
		err := CheckQuotaItems(items, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quotas[1]")
		assert.Contains(t, err.Error(), string(AccountingQuotaTotal))
		assert.Contains(t, err.Error(), string(AccountingQuotaValueTypeRequest))

		// The combination check applies to updates as well.
		err = CheckQuotaItems(items, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quotas[1]")
	})

	t.Run("create mode enforces value type cardinality", func(t *testing.T) {
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeFee, Quota: 50},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTotal, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
		}
		err := CheckQuotaItems(items, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), string(AccountingQuotaValueTypeFee))
	})

	t.Run("update mode skips value type cardinality", func(t *testing.T) {
		// Three fee records are fine on update: reconciliation may produce
		// multiple rows for an existing key, and the tx-level CheckQuotaSet
		// remains the cardinality gatekeeper.
		items := []UpdateAPIKeyQuotaItem{
			{QuotaType: AccountingQuotaTypeDaily, ValueType: AccountingQuotaValueTypeFee, Quota: 50},
			{QuotaType: AccountingQuotaTypeMonthly, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
			{QuotaType: AccountingQuotaTotal, ValueType: AccountingQuotaValueTypeFee, Quota: 100},
		}
		assert.NoError(t, CheckQuotaItems(items, false))
	})
}
