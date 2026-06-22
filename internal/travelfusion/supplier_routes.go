package travelfusion

import (
	"encoding/xml"
	"strings"
)

type SupplierRoutesResult struct {
	AirportRoutes []string
	CityRoutes    []string
}

type commandListGetBranchSupplierList struct {
	XMLName               xml.Name                     `xml:"CommandList"`
	GetBranchSupplierList getBranchSupplierListCommand `xml:"GetBranchSupplierList"`
}

type getBranchSupplierListCommand struct {
	XmlLoginID string `xml:"XmlLoginId"`
	LoginID    string `xml:"LoginId"`
}

type commandListGetBranchSupplierListResponse struct {
	XMLName               xml.Name                      `xml:"CommandList"`
	GetBranchSupplierList getBranchSupplierListResponse `xml:"GetBranchSupplierList"`
}

type getBranchSupplierListResponse struct {
	Suppliers []string `xml:"BranchSupplierList>Supplier"`
}

type commandListListSupplierRoutes struct {
	XMLName            xml.Name                  `xml:"CommandList"`
	ListSupplierRoutes listSupplierRoutesCommand `xml:"ListSupplierRoutes"`
}

type listSupplierRoutesCommand struct {
	XmlLoginID              string `xml:"XmlLoginId"`
	LoginID                 string `xml:"LoginId"`
	Supplier                string `xml:"Supplier"`
	OneWayOnlyAirportRoutes bool   `xml:"OneWayOnlyAirportRoutes"`
}

type commandListListSupplierRoutesResponse struct {
	XMLName            xml.Name                   `xml:"CommandList"`
	ListSupplierRoutes listSupplierRoutesResponse `xml:"ListSupplierRoutes"`
}

type listSupplierRoutesResponse struct {
	AirportRoutes string `xml:"RouteList>AirportRoutes"`
	CityRoutes    string `xml:"RouteList>CityRoutes"`
}

func buildGetBranchSupplierListXML(xmlLoginID, loginID string) ([]byte, error) {
	return xml.Marshal(commandListGetBranchSupplierList{
		GetBranchSupplierList: getBranchSupplierListCommand{
			XmlLoginID: xmlLoginID,
			LoginID:    loginID,
		},
	})
}

func buildListSupplierRoutesXML(xmlLoginID, loginID, supplier string, oneWayOnlyAirportRoutes bool) ([]byte, error) {
	return xml.Marshal(commandListListSupplierRoutes{
		ListSupplierRoutes: listSupplierRoutesCommand{
			XmlLoginID:              xmlLoginID,
			LoginID:                 loginID,
			Supplier:                supplier,
			OneWayOnlyAirportRoutes: oneWayOnlyAirportRoutes,
		},
	})
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
