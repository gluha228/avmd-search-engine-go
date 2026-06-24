package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
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

func newGetCurrenciesCommand(xmlLoginID, loginID string) getCurrenciesCommand {
	return getCurrenciesCommand{
		XmlLoginID: xmlLoginID,
		LoginID:    loginID,
	}
}

func (c *Client) getCurrencies(ctx context.Context, cmd getCurrenciesCommand) (getCurrenciesResponse, error) {
	payload, err := xml.Marshal(commandListGetCurrencies{GetCurrencies: cmd})
	if err != nil {
		return getCurrenciesResponse{}, fmt.Errorf("build get currencies request: %w", err)
	}
	body, err := c.postXML(ctx, "GetCurrencies", payload)
	if err != nil {
		return getCurrenciesResponse{}, fmt.Errorf("get currencies: %w", err)
	}
	var resp commandListGetCurrenciesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return getCurrenciesResponse{}, fmt.Errorf("parse get currencies response: %w", err)
	}
	return resp.GetCurrencies, nil
}
