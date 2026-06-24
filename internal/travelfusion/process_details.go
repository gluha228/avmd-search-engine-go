package travelfusion

import (
	"context"
	"strings"
)

type ProcessDetailsRequest struct {
	RoutingID string
	OutwardID string
	ReturnID  string
}

type ProcessDetailsResult struct {
	RoutingID          string
	RequiredParameters []RequiredParameter
}

type RequiredParameter struct {
	Name                string
	Value               string
	Type                string
	DisplayText         string
	PerPassenger        *bool
	IsOptional          *bool
	IsSometimesRequired bool
}

func (c *Client) ProcessDetails(ctx context.Context, req ProcessDetailsRequest) (*ProcessDetailsResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	resp, err := c.processDetails(ctx, newProcessDetailsCommand(c.xmlLoginID, c.loginID, req))
	if err != nil {
		return nil, err
	}
	return mapProcessDetails(resp), nil
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
