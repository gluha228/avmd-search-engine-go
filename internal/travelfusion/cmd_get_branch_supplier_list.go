package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
)

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

func newGetBranchSupplierListCommand(xmlLoginID, loginID string) getBranchSupplierListCommand {
	return getBranchSupplierListCommand{
		XmlLoginID: xmlLoginID,
		LoginID:    loginID,
	}
}

func (c *Client) getBranchSupplierList(ctx context.Context, cmd getBranchSupplierListCommand) (getBranchSupplierListResponse, error) {
	payload, err := xml.Marshal(commandListGetBranchSupplierList{GetBranchSupplierList: cmd})
	if err != nil {
		return getBranchSupplierListResponse{}, fmt.Errorf("build get branch supplier list request: %w", err)
	}
	body, err := c.postXML(ctx, "GetBranchSupplierList", payload)
	if err != nil {
		return getBranchSupplierListResponse{}, fmt.Errorf("get branch supplier list: %w", err)
	}
	var resp commandListGetBranchSupplierListResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return getBranchSupplierListResponse{}, fmt.Errorf("parse get branch supplier list response: %w", err)
	}
	return resp.GetBranchSupplierList, nil
}
