package money

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewMoney(t *testing.T) {
	m, err := NewMoney(1000, CurrencyCNY) // 10 yuan (1000 cents)
	require.NoError(t, err)

	assert.Equal(t, int64(1000), m.GetAmount(), "Amount should be 1000")
	assert.Equal(t, CurrencyCNY, m.GetCurrency(), "Currency should be CNY")
}

func TestMoney_Add(t *testing.T) {
	m1, err := NewMoney(1000, CurrencyCNY) // 10 yuan
	require.NoError(t, err)
	m2, err := NewMoney(500, CurrencyCNY) // 5 yuan
	require.NoError(t, err)

	// Test valid addition
	result, err := m1.Add(m2)
	require.NoError(t, err)
	assert.Equal(t, int64(1500), result.GetAmount(), "Addition result should be 1500")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test currency mismatch
	m3, _ := NewMoney(1000, CurrencyUSD)
	_, err = m1.Add(m3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency mismatch", "Error should indicate currency mismatch")
}

func TestMoney_Sub(t *testing.T) {
	m1, er := NewMoney(1000, CurrencyCNY) // 10 yuan
	require.NoError(t, er)
	m2, er := NewMoney(500, CurrencyCNY) // 5 yuan
	require.NoError(t, er)

	// Test valid subtraction
	result, err := m1.Sub(m2)
	require.NoError(t, err)
	assert.Equal(t, int64(500), result.GetAmount(), "Subtraction result should be 500")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test negative result
	result, err = m2.Sub(m1)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), result.GetAmount(), "Result should be -500")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test currency mismatch
	m3, _ := NewMoney(1000, CurrencyUSD)
	_, err = m1.Sub(m3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency mismatch", "Error should indicate currency mismatch")
}

func TestMoney_Multiply(t *testing.T) {
	m, _ := NewMoney(1000, CurrencyCNY) // 10 yuan

	// Test valid multiplication
	result := m.Multiply(3)
	assert.Equal(t, int64(3000), result.GetAmount(), "Multiplication result should be 3000")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test multiplication by zero
	result = m.Multiply(0)
	assert.Equal(t, int64(0), result.GetAmount(), "Multiplication result should be 0")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test multiplication by negative number
	result = m.Multiply(-2)
	assert.Equal(t, int64(-2000), result.GetAmount(), "Multiplication result should be -2000")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")
}

func TestMoney_Divide(t *testing.T) {
	m, _ := NewMoney(1000, CurrencyCNY) // 10 yuan

	// Test valid division
	result, err := m.Divide(2)
	require.NoError(t, err)
	assert.Equal(t, int64(500), result.GetAmount(), "Division result should be 500")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")

	// Test division by zero
	_, err = m.Divide(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot divide by zero", "Error should indicate division by zero")

	// Test division resulting in fractional values (rounding handled)
	result, err = m.Divide(3)
	require.NoError(t, err)
	assert.Equal(t, int64(333), result.GetAmount(), "Division result should truncate towards zero")
	assert.Equal(t, CurrencyCNY, result.GetCurrency(), "Currency should remain CNY")
}

func TestMoney_Format(t *testing.T) {
	m, _ := NewMoney(123456, CurrencyCNY) // 1234.56 yuan
	assert.Equal(t, "1234.56 CNY", m.Format(), "Formatted string should match '1234.56 CNY'")

	m, _ = NewMoney(-789, CurrencyUSD) // -7.89 dollars
	assert.Equal(t, "-7.89 USD", m.Format(), "Formatted string should match '-7.89 USD'")

	m, _ = NewMoney(0, CurrencyEUR) // 0 euros
	assert.Equal(t, "0.00 EUR", m.Format(), "Formatted string should match '0.00 EUR'")
}

func TestMoney_ToYuanString(t *testing.T) {
	// Test valid CNY amount within range
	m1, err := NewMoney(1000, CurrencyCNY) // 10 yuan
	require.NoError(t, err)
	yuanStr, err := m1.ToYuanString()
	require.NoError(t, err)
	assert.Equal(t, "10.00", yuanStr, "Expected yuan string to be '10.00'")

	// Test amount exactly at minimum allowed (0.01 yuan)
	m2, err := NewMoney(1, CurrencyCNY) // 0.01 yuan
	require.NoError(t, err)
	yuanStr, err = m2.ToYuanString()
	require.NoError(t, err)
	assert.Equal(t, "0.01", yuanStr, "Expected yuan string to be '0.01'")

	// Test amount exactly at maximum allowed (100,000,000 yuan)
	maxFen := int64(100000000 * 100) // 100,000,000 yuan in fen
	m3, err := NewMoney(maxFen, CurrencyCNY)
	require.NoError(t, err)
	yuanStr, err = m3.ToYuanString()
	require.NoError(t, err)
	assert.Equal(t, "100000000.00", yuanStr, "Expected yuan string to be '100000000.00'")

	// Test amount less than minimum allowed (less than 0.01 yuan)
	m4, err := NewMoney(0, CurrencyCNY)
	require.NoError(t, err)
	_, err = m4.ToYuanString()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of allowed range", "Error should indicate amount is out of allowed range")

	// Test amount greater than maximum allowed (more than 100,000,000 yuan)
	m5, err := NewMoney(maxFen+1, CurrencyCNY)
	require.NoError(t, err)
	_, err = m5.ToYuanString()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of allowed range", "Error should indicate amount is out of allowed range")

	// Test negative amount
	m6, err := NewMoney(-100, CurrencyCNY) // -1 yuan
	require.NoError(t, err)
	_, err = m6.ToYuanString()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of allowed range", "Error should indicate amount is out of allowed range")

	// Test currency mismatch
	m7, err := NewMoney(1000, CurrencyUSD)
	require.NoError(t, err)
	_, err = m7.ToYuanString()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency mismatch", "Error should indicate currency mismatch")
}
