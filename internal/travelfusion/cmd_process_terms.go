package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
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

func newProcessTermsCommand(xmlLoginID, loginID string, req ProcessTermsRequest) processTermsCommand {
	return processTermsCommand{
		XmlLoginID:     xmlLoginID,
		LoginID:        loginID,
		RoutingID:      req.RoutingID,
		OutwardID:      req.OutwardID,
		ReturnID:       req.ReturnID,
		BookingProfile: mapProcessTermsProfile(req.BookingProfile),
	}
}

func (c *Client) processTerms(ctx context.Context, cmd processTermsCommand) (processTermsResponse, error) {
	payload, err := xml.Marshal(commandListProcessTerms{ProcessTerms: cmd})
	if err != nil {
		return processTermsResponse{}, fmt.Errorf("build process terms request: %w", err)
	}
	body, err := c.postXML(ctx, "ProcessTerms", payload)
	if err != nil {
		return processTermsResponse{}, fmt.Errorf("process terms: %w", err)
	}
	var resp commandListProcessTermsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return processTermsResponse{}, fmt.Errorf("parse process terms response: %w", err)
	}
	return resp.ProcessTerms, nil
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
