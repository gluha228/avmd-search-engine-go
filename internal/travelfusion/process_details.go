package travelfusion

import (
	"encoding/xml"
	"strings"
)

type commandListProcessDetails struct {
	XMLName        xml.Name              `xml:"CommandList"`
	ProcessDetails processDetailsCommand `xml:"ProcessDetails"`
}

type processDetailsCommand struct {
	XmlLoginID            string `xml:"XmlLoginId"`
	LoginID               string `xml:"LoginId"`
	RoutingID             string `xml:"RoutingId"`
	OutwardID             string `xml:"OutwardId"`
	ReturnID              string `xml:"ReturnId,omitempty"`
	HandoffParametersOnly bool   `xml:"HandoffParametersOnly,omitempty"`
}

type commandListProcessDetailsResponse struct {
	ProcessDetails processDetailsResponse `xml:"ProcessDetails"`
}

type processDetailsResponse struct {
	RoutingID string               `xml:"RoutingId"`
	Router    processDetailsRouter `xml:"Router"`
}

type processDetailsRouter struct {
	RequiredParameters []requiredParameterXML `xml:"RequiredParameterList>RequiredParameter"`
}

type requiredParameterXML struct {
	Name                string  `xml:"Name"`
	Value               string  `xml:"Value"`
	Type                string  `xml:"Type"`
	DisplayText         string  `xml:"DisplayText"`
	PerPassenger        *bool   `xml:"PerPassenger"`
	IsOptional          *bool   `xml:"IsOptional"`
	IsSometimesRequired *string `xml:"IsSometimesRequired"`
}

func buildProcessDetailsXML(xmlLoginID, loginID string, req ProcessDetailsRequest) ([]byte, error) {
	return xml.Marshal(commandListProcessDetails{
		ProcessDetails: processDetailsCommand{
			XmlLoginID: xmlLoginID,
			LoginID:    loginID,
			RoutingID:  req.RoutingID,
			OutwardID:  req.OutwardID,
			ReturnID:   req.ReturnID,
		},
	})
}

func mapProcessDetails(resp processDetailsResponse) *ProcessDetailsResult {
	result := &ProcessDetailsResult{
		RoutingID: strings.TrimSpace(resp.RoutingID),
	}
	for _, raw := range resp.Router.RequiredParameters {
		name := strings.TrimSpace(raw.Name)
		if name == "" {
			continue
		}
		result.RequiredParameters = append(result.RequiredParameters, RequiredParameter{
			Name:                name,
			Value:               strings.TrimSpace(raw.Value),
			Type:                normalizeRequiredParameterType(raw.Type),
			DisplayText:         strings.TrimSpace(raw.DisplayText),
			PerPassenger:        raw.PerPassenger,
			IsOptional:          raw.IsOptional,
			IsSometimesRequired: raw.IsSometimesRequired != nil,
		})
	}
	return result
}

func normalizeRequiredParameterType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text":
		return "TEXT"
	case "boolean":
		return "BOOLEAN"
	case "formatted_text":
		return "FORMATTED_TEXT"
	case "value_select":
		return "VALUE_SELECT"
	case "multi_select":
		return "MULTI_SELECT"
	case "custom":
		return "CUSTOM"
	case "notice":
		return "NOTICE"
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}
