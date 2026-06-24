package travelfusion

import (
	"context"
	"strings"
)

type ProcessTermsRequest struct {
	RoutingID      string
	OutwardID      string
	ReturnID       string
	BookingProfile BookingProfile
}

type BookingProfile struct {
	Travellers               []Traveller
	ContactDetails           ContactDetails
	CustomSupplierParameters []CustomSupplierParameter
}

type Traveller struct {
	Age                      int
	Name                     Name
	CustomSupplierParameters []CustomSupplierParameter
}

type Name struct {
	Title     string
	NameParts []string
}

type ContactDetails struct {
	Name        Name
	Address     Address
	MobilePhone Phone
	Email       string
}

type Address struct {
	City        string
	Street      string
	CountryCode string
	Postcode    string
	Province    string
}

type Phone struct {
	InternationalCode string
	Number            string
}

type CustomSupplierParameter struct {
	Name  string
	Value string
}

type ProcessTermsResult struct {
	RoutingID                           string
	TFBookingReference                  string
	FinalAmount                         *float64
	FinalCurrency                       string
	SupplierVisualAuthorisationImageURL string
	SupplierResponses                   []SupplierResponse
}

type SupplierResponse struct {
	Name string
	Type string
	Data string
}

func (c *Client) ProcessTerms(ctx context.Context, req ProcessTermsRequest) (*ProcessTermsResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	resp, err := c.processTerms(ctx, newProcessTermsCommand(c.xmlLoginID, c.loginID, req))
	if err != nil {
		return nil, err
	}
	return mapProcessTerms(resp), nil
}

func mapProcessTerms(resp processTermsResponse) *ProcessTermsResult {
	result := &ProcessTermsResult{
		RoutingID:                           strings.TrimSpace(resp.RoutingID),
		TFBookingReference:                  strings.TrimSpace(resp.TFBookingReference),
		SupplierVisualAuthorisationImageURL: strings.TrimSpace(resp.SupplierVisualAuthorisationImageURL),
	}
	if resp.Price.Amount != 0 {
		result.FinalAmount = &resp.Price.Amount
		result.FinalCurrency = strings.TrimSpace(resp.Price.Currency)
	} else if len(resp.Router.GroupList) > 0 && resp.Router.GroupList[0].Price.Amount != 0 {
		amount := resp.Router.GroupList[0].Price.Amount
		result.FinalAmount = &amount
		result.FinalCurrency = strings.TrimSpace(resp.Router.GroupList[0].Price.Currency)
	}
	for _, supplierResponse := range resp.SupplierResponseList {
		result.SupplierResponses = append(result.SupplierResponses, SupplierResponse{
			Name: strings.TrimSpace(supplierResponse.Name),
			Type: strings.TrimSpace(supplierResponse.Type),
			Data: supplierResponse.Data,
		})
	}
	return result
}
