package travelfusion

import (
	"context"
	"strings"
)

type Currency struct {
	Name    string
	Code    string
	USDRate float64
}

func (c *Client) GetCurrencies(ctx context.Context) (map[string]Currency, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	resp, err := c.getCurrencies(ctx, newGetCurrenciesCommand(c.xmlLoginID, c.loginID))
	if err != nil {
		return nil, err
	}
	return mapCurrencies(resp), nil
}

func mapCurrencies(resp getCurrenciesResponse) map[string]Currency {
	result := make(map[string]Currency, len(resp.CurrencyList))
	for _, currency := range resp.CurrencyList {
		code := strings.ToUpper(strings.TrimSpace(currency.Code))
		if code == "" {
			continue
		}
		result[code] = Currency{
			Name:    strings.TrimSpace(currency.Name),
			Code:    code,
			USDRate: currency.USDRate,
		}
	}
	return result
}
