package domain

// Currency holds metadata for a supported ISO 4217 currency.
type Currency struct {
	Code          string `json:"code"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	DecimalPlaces int    `json:"decimal_places"`
}

// Currencies is the registry of the 15 supported currencies.
// DecimalPlaces drives the float64 ↔ int64 minor-unit conversion at the API boundary.
var Currencies = map[string]Currency{
	"USD": {"USD", "$",   "US Dollar",         2},
	"EUR": {"EUR", "€",   "Euro",              2},
	"GBP": {"GBP", "£",   "British Pound",     2},
	"JPY": {"JPY", "¥",   "Japanese Yen",      0},
	"CAD": {"CAD", "CA$", "Canadian Dollar",   2},
	"AUD": {"AUD", "A$",  "Australian Dollar", 2},
	"CHF": {"CHF", "Fr",  "Swiss Franc",       2},
	"CNY": {"CNY", "¥",   "Chinese Yuan",      2},
	"INR": {"INR", "₹",   "Indian Rupee",      2},
	"SGD": {"SGD", "S$",  "Singapore Dollar",  2},
	"HKD": {"HKD", "HK$", "Hong Kong Dollar",  2},
	"NOK": {"NOK", "kr",  "Norwegian Krone",   2},
	"SEK": {"SEK", "kr",  "Swedish Krona",     2},
	"MXN": {"MXN", "MX$", "Mexican Peso",      2},
	"BRL": {"BRL", "R$",  "Brazilian Real",    2},
}

// IsSupported returns true if code is one of the 15 supported currencies.
func IsSupported(code string) bool {
	_, ok := Currencies[code]
	return ok
}

// MinorUnitFactor returns 10^decimal_places for the given currency code.
// Examples: USD → 100, EUR → 100, JPY → 1.
// Falls back to 100 for unknown codes (safe default).
func MinorUnitFactor(code string) int64 {
	c, ok := Currencies[code]
	if !ok {
		return 100
	}
	factor := int64(1)
	for i := 0; i < c.DecimalPlaces; i++ {
		factor *= 10
	}
	return factor
}
