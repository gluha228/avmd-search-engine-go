package travelfusion

import (
	"encoding/xml"
	"strings"
)

type commandListProcessTerms struct {
	XMLName      xml.Name            `xml:"CommandList"`
	ProcessTerms processTermsCommand `xml:"ProcessTerms"`
}

type processTermsCommand struct {
	XmlLoginID     string                 `xml:"XmlLoginId"`
	LoginID        string                 `xml:"LoginId"`
	RoutingID      string                 `xml:"RoutingId"`
	OutwardID      string                 `xml:"OutwardId"`
	ReturnID       string                 `xml:"ReturnId,omitempty"`
	BookingProfile processTermsProfileXML `xml:"BookingProfile"`
}

type processTermsProfileXML struct {
	TravellerList            processTermsTravellerList   `xml:"TravellerList"`
	ContactDetails           processTermsContactXML      `xml:"ContactDetails"`
	CustomSupplierParameters customSupplierParameterList `xml:"CustomSupplierParameterList"`
}

type processTermsTravellerList struct {
	Travellers []processTermsTravellerXML `xml:"Traveller"`
}

type processTermsTravellerXML struct {
	Age                      int                         `xml:"Age"`
	Name                     processTermsNameXML         `xml:"Name"`
	CustomSupplierParameters customSupplierParameterList `xml:"CustomSupplierParameterList"`
}

type processTermsNameXML struct {
	Title        string       `xml:"Title"`
	NamePartList namePartList `xml:"NamePartList"`
}

type namePartList struct {
	NameParts []string `xml:"NamePart"`
}

type processTermsContactXML struct {
	Name        processTermsNameXML  `xml:"Name"`
	Address     processTermsAddress  `xml:"Address"`
	MobilePhone processTermsPhoneXML `xml:"MobilePhone"`
	Email       string               `xml:"Email"`
}

type processTermsAddress struct {
	Street      string `xml:"Street"`
	City        string `xml:"City"`
	Province    string `xml:"Province"`
	Postcode    string `xml:"Postcode"`
	CountryCode string `xml:"CountryCode"`
}

type processTermsPhoneXML struct {
	InternationalCode string `xml:"InternationalCode"`
	Number            string `xml:"Number"`
}

type customSupplierParameterList struct {
	Parameters []customSupplierParameterXML `xml:"CustomSupplierParameter"`
}

type customSupplierParameterXML struct {
	Name  string `xml:"Name"`
	Value string `xml:"Value"`
}

type commandListProcessTermsResponse struct {
	ProcessTerms processTermsResponse `xml:"ProcessTerms"`
}

type processTermsResponse struct {
	RoutingID                           string                `xml:"RoutingId"`
	TFBookingReference                  string                `xml:"TFBookingReference"`
	Price                               price                 `xml:"Price"`
	Router                              processTermsRouter    `xml:"Router"`
	SupplierResponseList                []supplierResponseXML `xml:"SupplierResponseList>SupplierResponse"`
	SupplierVisualAuthorisationImageURL string                `xml:"SupplierVisualAuthorisationImageURL"`
}

type processTermsRouter struct {
	GroupList []processTermsGroup `xml:"GroupList>Group"`
}

type processTermsGroup struct {
	Price price `xml:"Price"`
}

type supplierResponseXML struct {
	Name string `xml:"Name"`
	Type string `xml:"Type"`
	Data string `xml:"Data"`
}

func buildProcessTermsXML(xmlLoginID, loginID string, req ProcessTermsRequest) ([]byte, error) {
	return xml.Marshal(commandListProcessTerms{
		ProcessTerms: processTermsCommand{
			XmlLoginID:     xmlLoginID,
			LoginID:        loginID,
			RoutingID:      req.RoutingID,
			OutwardID:      req.OutwardID,
			ReturnID:       req.ReturnID,
			BookingProfile: mapProcessTermsProfile(req.BookingProfile),
		},
	})
}

func mapProcessTermsProfile(profile BookingProfile) processTermsProfileXML {
	travellers := make([]processTermsTravellerXML, len(profile.Travellers))
	for i := range profile.Travellers {
		travellers[i] = processTermsTravellerXML{
			Age:                      profile.Travellers[i].Age,
			Name:                     mapProcessTermsName(profile.Travellers[i].Name),
			CustomSupplierParameters: mapCustomSupplierParameters(profile.Travellers[i].CustomSupplierParameters),
		}
	}
	return processTermsProfileXML{
		TravellerList:            processTermsTravellerList{Travellers: travellers},
		ContactDetails:           mapProcessTermsContact(profile.ContactDetails),
		CustomSupplierParameters: mapCustomSupplierParameters(profile.CustomSupplierParameters),
	}
}

func mapProcessTermsName(name Name) processTermsNameXML {
	return processTermsNameXML{
		Title:        strings.TrimSpace(name.Title),
		NamePartList: namePartList{NameParts: name.NameParts},
	}
}

func mapProcessTermsContact(contact ContactDetails) processTermsContactXML {
	return processTermsContactXML{
		Name: mapProcessTermsName(contact.Name),
		Address: processTermsAddress{
			Street:      contact.Address.Street,
			City:        contact.Address.City,
			Province:    contact.Address.Province,
			Postcode:    contact.Address.Postcode,
			CountryCode: contact.Address.CountryCode,
		},
		MobilePhone: processTermsPhoneXML{
			InternationalCode: contact.MobilePhone.InternationalCode,
			Number:            contact.MobilePhone.Number,
		},
		Email: contact.Email,
	}
}

func mapCustomSupplierParameters(src []CustomSupplierParameter) customSupplierParameterList {
	params := make([]customSupplierParameterXML, 0, len(src))
	for _, param := range src {
		name := strings.TrimSpace(param.Name)
		value := strings.TrimSpace(param.Value)
		if name == "" || value == "" {
			continue
		}
		params = append(params, customSupplierParameterXML{Name: name, Value: value})
	}
	return customSupplierParameterList{Parameters: params}
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
