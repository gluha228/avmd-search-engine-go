package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
)

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

func newListSupplierRoutesCommand(xmlLoginID, loginID, supplier string, oneWayOnlyAirportRoutes bool) listSupplierRoutesCommand {
	return listSupplierRoutesCommand{
		XmlLoginID:              xmlLoginID,
		LoginID:                 loginID,
		Supplier:                supplier,
		OneWayOnlyAirportRoutes: oneWayOnlyAirportRoutes,
	}
}

func (c *Client) listSupplierRoutes(ctx context.Context, cmd listSupplierRoutesCommand) (listSupplierRoutesResponse, error) {
	payload, err := xml.Marshal(commandListListSupplierRoutes{ListSupplierRoutes: cmd})
	if err != nil {
		return listSupplierRoutesResponse{}, fmt.Errorf("build list supplier routes request: %w", err)
	}
	body, err := c.postXML(ctx, "ListSupplierRoutes", payload)
	if err != nil {
		return listSupplierRoutesResponse{}, fmt.Errorf("list supplier routes: %w", err)
	}
	var resp commandListListSupplierRoutesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return listSupplierRoutesResponse{}, fmt.Errorf("parse list supplier routes response: %w", err)
	}
	return resp.ListSupplierRoutes, nil
}
