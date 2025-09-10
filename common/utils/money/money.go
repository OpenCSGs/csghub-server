package money

import (
	"errors"
	"fmt"
	"math/big"
)

// Money struct to handle currency values safely
type Money struct {
	amount   int64    // Amount stored in the smallest unit (e.g., cents)
	currency Currency // 3-letter ISO currency code (e.g., "CNY", "USD")
}

// NewMoney creates a new Money instance
func NewMoney(amount int64, currency Currency) (*Money, error) {
	if !isValidCurrency(currency) {
		return nil, errors.New("invalid currency")
	}
	return &Money{
		amount:   amount,
		currency: currency,
	}, nil
}

// NewMoneyFromYuan converts a yuan amount into a Money object in fen.
//
// Parameters:
// - yuanAmount (float64): The amount in yuan to convert.
//
// Returns:
// - *Money: A Money instance representing the amount in fen.
// - error: An error if the conversion fails.
//
// Note:
// - This method assumes that 1 yuan equals 100 fen.
// - Be cautious with floating-point precision when dealing with currency amounts.
func NewMoneyFromYuan(yuanAmount float64) (*Money, error) {
	fenAmount := int64(yuanAmount * 100)
	return NewMoney(fenAmount, CurrencyCNY)
}

func (m *Money) validateSameCurrency(other *Money) error {
	if m.currency != other.currency {
		return fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	return nil
}

// Add adds another Money value (must have the same currency)
func (m *Money) Add(other *Money) (*Money, error) {
	if err := m.validateSameCurrency(other); err != nil {
		return nil, err
	}
	newAmount := m.amount + other.amount
	return &Money{amount: newAmount, currency: m.currency}, nil
}

// Sub subtracts another Money value (must have the same currency)
func (m *Money) Sub(other *Money) (*Money, error) {
	if err := m.validateSameCurrency(other); err != nil {
		return nil, err
	}
	newAmount := m.amount - other.amount
	return &Money{amount: newAmount, currency: m.currency}, nil
}

// Multiply multiplies the Money amount by a scalar
func (m *Money) Multiply(factor int64) *Money {
	newAmount := m.amount * factor
	return &Money{amount: newAmount, currency: m.currency}
}

// Divide divides the Money amount by a scalar
func (m *Money) Divide(divisor int64) (*Money, error) {
	if divisor == 0 {
		return nil, fmt.Errorf("cannot divide by zero")
	}
	newAmount := m.amount / divisor
	return &Money{amount: newAmount, currency: m.currency}, nil
}

// Format returns the formatted string representation of Money
func (m *Money) Format() string {
	// Convert to the "major" unit (e.g., dollars from cents)
	major := big.NewRat(m.amount, 100)
	return fmt.Sprintf("%s %s", major.FloatString(2), m.currency)
}

// GetAmount returns the amount in the smallest unit (e.g., cents)
func (m *Money) GetAmount() int64 {
	return m.amount
}

// GetCurrency returns the currency code
func (m *Money) GetCurrency() Currency {
	return m.currency
}

// toYuanRat is an internal helper method that converts the Money amount to a big.Rat representing yuan.
// It validates that the currency is CNY and checks if the amount is within Alipay's required range.
//
// Returns:
// - *big.Rat: A big.Rat representing the amount in yuan.
// - error: An error if the currency is not CNY or the amount is out of range.
func (m *Money) toYuanRat() (*big.Rat, error) {
	// Validate that the currency is CNY
	if m.currency != CurrencyCNY {
		return nil, fmt.Errorf("currency mismatch: expected CNY, got %s", m.currency)
	}

	// Create a big.Rat representing the amount in fen
	amountRat := new(big.Rat).SetInt64(m.amount)

	// Conversion factor: 1 yuan = 100 fen
	factor := big.NewRat(1, 100)

	// Convert from fen to yuan
	yuanRat := new(big.Rat).Mul(amountRat, factor)

	// Alipay requires the amount to be within [0.01, 100000000]
	minAmount := big.NewRat(1, 100)       // 0.01
	maxAmount := big.NewRat(100000000, 1) // 100000000

	if yuanRat.Cmp(minAmount) < 0 || yuanRat.Cmp(maxAmount) > 0 {
		return nil, fmt.Errorf("amount %s is out of allowed range [0.01, 100000000]", yuanRat.FloatString(2))
	}

	return yuanRat, nil
}

// ToYuanString converts the Money amount to a string representing yuan, formatted to two decimal places.
func (m *Money) ToYuanString() (string, error) {
	yuanRat, err := m.toYuanRat()
	if err != nil {
		return "", err
	}
	// Format the big.Rat to a string with two decimal places
	return yuanRat.FloatString(2), nil
}

// ToYuanFloat converts the Money amount to a float64 representing yuan.
func (m *Money) ToYuanFloat() (float64, error) {
	yuanRat, err := m.toYuanRat()
	if err != nil {
		return 0, err
	}
	// Convert the big.Rat to a float64
	yuanFloat, _ := yuanRat.Float64()
	return yuanFloat, nil
}
