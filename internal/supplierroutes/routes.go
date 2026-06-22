package supplierroutes

import (
	"sort"
	"strings"
)

func addRoutes(target map[string]struct{}, routes []string) {
	for _, route := range routes {
		route = normalizeRouteCode(route)
		if len(route) == 6 {
			target[route] = struct{}{}
		}
	}
}

func setToSortedSlice(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func knownAirportsFromRoutes(routes []string) []string {
	seen := make(map[string]struct{}, len(routes)*2)
	for _, route := range routes {
		route = normalizeRouteCode(route)
		if len(route) != 6 {
			continue
		}
		seen[route[:3]] = struct{}{}
		seen[route[3:6]] = struct{}{}
	}
	return setToSortedSlice(seen)
}

func normalizeRouteCode(route string) string {
	return strings.ToUpper(strings.TrimSpace(route))
}

func normalizeAirportCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
