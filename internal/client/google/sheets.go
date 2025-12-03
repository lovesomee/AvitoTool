package google

import (
	"fmt"
	"google.golang.org/api/sheets/v4"
)

func (c *Client) UpdateSheet(shopRange string, values [][]interface{}) error {
	vr := &sheets.ValueRange{Values: values}
	_, err := c.Service.Spreadsheets.Values.Update(c.SpreadsheetID, shopRange, vr).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to update sheet: %w", err)
	}
	return nil
}

func (c *Client) BatchUpdate(data map[string][][]interface{}) error {
	requests := []*sheets.ValueRange{}
	for r, values := range data {
		requests = append(requests, &sheets.ValueRange{
			Range:  r,
			Values: values,
		})
	}
	_, err := c.Service.Spreadsheets.Values.BatchUpdate(c.SpreadsheetID, &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "RAW",
		Data:             requests,
	}).Do()
	if err != nil {
		return fmt.Errorf("batch update failed: %w", err)
	}
	return nil
}

func (c *Client) ReadRange(r string) ([][]interface{}, error) {
	resp, err := c.Service.Spreadsheets.Values.Get(c.SpreadsheetID, r).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to read range %s: %w", r, err)
	}
	return resp.Values, nil
}
