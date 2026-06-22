package travelfusion

import (
	"encoding/xml"
	"strings"
)

type commandListGetCurrencies struct {
	XMLName       xml.Name             `xml:"CommandList"`
	GetCurrencies getCurrenciesCommand `xml:"GetCurrencies"`
}

type getCurrenciesCommand struct {
	XmlLoginID string `xml:"XmlLoginId"`
	LoginID    string `xml:"LoginId"`
}

type commandListGetCurrenciesResponse struct {
	XMLName       xml.Name              `xml:"CommandList"`
	GetCurrencies getCurrenciesResponse `xml:"GetCurrencies"`
}

type getCurrenciesResponse struct {
	CurrencyList []xmlCurrency `xml:"CurrencyList>Currency"`
}

type xmlCurrency struct {
	Name    string  `xml:"Name"`
	Code    string  `xml:"Code"`
	USDRate float64 `xml:"UsdRate"`
}

func buildGetCurrenciesXML(xmlLoginID, loginID string) ([]byte, error) {
	return xml.Marshal(commandListGetCurrencies{
		GetCurrencies: getCurrenciesCommand{
			XmlLoginID: xmlLoginID,
			LoginID:    loginID,
		},
	})
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
