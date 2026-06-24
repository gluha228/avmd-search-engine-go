package flights

import (
	"avmd-search-engine-go/internal/db"
	"context"
	"strings"
)

type SQLCAirportLookup struct {
	queries *db.Queries
}

func NewSQLCAirportLookup(dbtx db.DBTX) *SQLCAirportLookup {
	return &SQLCAirportLookup{queries: db.New(dbtx)}
}

func (l *SQLCAirportLookup) FlightAirportsByIATACodes(ctx context.Context, codes []string, locale string) (map[string]FlightAirport, error) {
	rows, err := l.queries.GetFlightAirportsByIATACodes(ctx, db.GetFlightAirportsByIATACodesParams{
		Locale:    normalizeAirportLocale(locale),
		IataCodes: codes,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]FlightAirport, len(rows))
	for _, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		result[code] = FlightAirport{
			Code:     code,
			CityName: row.CityName,
		}
	}
	return result, nil
}

func normalizeAirportLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if strings.HasPrefix(locale, "ru") {
		return "ru"
	}
	if strings.HasPrefix(locale, "ro") {
		return "ro"
	}
	return "en"
}
