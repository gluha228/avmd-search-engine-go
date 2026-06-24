package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
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

func newProcessDetailsCommand(xmlLoginID, loginID string, req ProcessDetailsRequest) processDetailsCommand {
	return processDetailsCommand{
		XmlLoginID: xmlLoginID,
		LoginID:    loginID,
		RoutingID:  req.RoutingID,
		OutwardID:  req.OutwardID,
		ReturnID:   req.ReturnID,
	}
}

func (c *Client) processDetails(ctx context.Context, cmd processDetailsCommand) (processDetailsResponse, error) {
	payload, err := xml.Marshal(commandListProcessDetails{ProcessDetails: cmd})
	if err != nil {
		return processDetailsResponse{}, fmt.Errorf("build process details request: %w", err)
	}
	body, err := c.postXML(ctx, "ProcessDetails", payload)
	if err != nil {
		return processDetailsResponse{}, fmt.Errorf("process details: %w", err)
	}
	var resp commandListProcessDetailsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return processDetailsResponse{}, fmt.Errorf("parse process details response: %w", err)
	}
	return resp.ProcessDetails, nil
}
