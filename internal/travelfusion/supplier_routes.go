package travelfusion

import (
	"context"
	"strings"
)

type SupplierRoutesResult struct {
	AirportRoutes []string
	CityRoutes    []string
}

func (c *Client) GetBranchSupplierList(ctx context.Context) ([]string, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	resp, err := c.getBranchSupplierList(ctx, newGetBranchSupplierListCommand(c.xmlLoginID, c.loginID))
	if err != nil {
		return nil, err
	}

	suppliers := make([]string, 0, len(resp.Suppliers))
	for _, supplier := range resp.Suppliers {
		supplier = strings.TrimSpace(supplier)
		if supplier != "" {
			suppliers = append(suppliers, supplier)
		}
	}
	return suppliers, nil
}

func (c *Client) ListSupplierRoutes(ctx context.Context, supplier string, oneWayOnlyAirportRoutes bool) (*SupplierRoutesResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	resp, err := c.listSupplierRoutes(ctx, newListSupplierRoutesCommand(c.xmlLoginID, c.loginID, supplier, oneWayOnlyAirportRoutes))
	if err != nil {
		return nil, err
	}
	return &SupplierRoutesResult{
		AirportRoutes: parseRouteCodes(resp.AirportRoutes),
		CityRoutes:    parseRouteCodes(resp.CityRoutes),
	}, nil
}

func parseRouteCodes(raw string) []string {
	fields := strings.Fields(raw)
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		route := strings.ToUpper(strings.TrimSpace(field))
		if len(route) != 6 {
			continue
		}
		if _, ok := seen[route]; ok {
			continue
		}
		seen[route] = struct{}{}
		result = append(result, route)
	}
	return result
}
