package constants

import "log"

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

func init() {
	log.Printf("[constants] AllowedAccountTypes loaded: %d types", len(AllowedAccountTypes))
	log.Printf("[constants] AllowedCurrencies loaded: %d currencies", len(AllowedCurrencies))
}
