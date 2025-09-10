package money

type Currency string

const (
	CurrencyCNY Currency = "CNY" // Chinese Yuan
	CurrencyUSD Currency = "USD" // US Dollar
	CurrencyEUR Currency = "EUR" // Euro
	CurrencyJPY Currency = "JPY" // Japanese Yen
	CurrencyGBP Currency = "GBP" // British Pound
)

func isValidCurrency(currency Currency) bool {
	switch currency {
	case CurrencyCNY, CurrencyUSD, CurrencyEUR, CurrencyJPY, CurrencyGBP:
		return true
	default:
		return false
	}
}
