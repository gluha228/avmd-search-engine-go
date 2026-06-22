package geo

import "strings"

var validRoutes = map[string]map[string]struct{}{
	"OTP": set("CLJ", "TSR", "OMR", "JFK", "ORD", "RMO", "DUB", "FRA", "DUS", "CDG", "BCN", "TLV", "BRU", "STN", "IST"),
	"IAS": set("DUB"),
	"CLJ": set("OTP", "JFK", "ORD", "RMO", "DUB", "FRA", "CDG", "BCN", "TLV", "BRU"),
	"BAY": set("BVA"),
	"TSR": set("OTP", "DUB", "FRA", "CDG", "BCN", "TLV", "BRU"),
	"OMR": set("DUB", "FRA", "CDG", "BGY", "BCN", "BRU", "STN"),
	"JFK": set("OTP", "CLJ", "OMR", "RMO", "TLV"),
	"ORD": set("OTP", "CLJ", "OMR", "RMO", "TLV"),
	"RMO": set("OTP", "CLJ", "OMR", "JFK", "ORD", "DUB", "FRA", "DUS", "HAM", "BVA", "CDG", "BGY", "VCE", "BCN", "TLV", "BRU", "STN", "IST"),
	"DUB": set("OTP", "IAS", "CLJ", "TSR", "OMR", "RMO"),
	"FRA": set("OTP", "CLJ", "TSR", "OMR", "RMO"),
	"DUS": set("OTP", "RMO"),
	"HAM": set("RMO"),
	"BVA": set("BAY", "RMO"),
	"CDG": set("OTP", "CLJ", "TSR", "OMR", "RMO"),
	"BGY": set("OMR", "RMO"),
	"VCE": set("RMO"),
	"BCN": set("OTP", "CLJ", "TSR", "OMR", "RMO"),
	"TLV": set("OTP", "CLJ", "TSR", "OMR", "JFK", "ORD", "RMO", "IST"),
	"BRU": set("OTP", "CLJ", "TSR", "OMR", "RMO"),
	"STN": set("OTP", "OMR", "RMO"),
	"IST": set("OTP", "RMO", "TLV"),
}

func isReachableAirport(airportCode string) bool {
	_, ok := validRoutes[strings.ToUpper(strings.TrimSpace(airportCode))]
	return ok
}

func isValidRoute(departureCode, arrivalCode string) bool {
	departureCode = strings.ToUpper(strings.TrimSpace(departureCode))
	arrivalCode = strings.ToUpper(strings.TrimSpace(arrivalCode))
	if departureCode == "" || arrivalCode == "" || departureCode == arrivalCode {
		return false
	}
	_, ok := validRoutes[departureCode][arrivalCode]
	return ok
}

func set(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
