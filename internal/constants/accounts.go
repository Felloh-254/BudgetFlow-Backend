package constants

var AllowedAccountTypes = map[string]bool{
	"bank":         true,
	"cash":         true,
	"mobile_money": true,
	"investment":   true,
}

var AllowedCurrencies = map[string]bool{
	"KES": true,
	"USD": true,
	"EUR": true,
	"GBP": true,
}
