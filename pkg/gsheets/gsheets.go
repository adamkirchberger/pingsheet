// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package gsheets

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// SheetRows is for unpacking multiple rows from Gsheets based on a slice of maps
type SheetRows []map[string]interface{}

// SheetRow is for unpacking single rows from Gsheets based on a slice of maps
type SheetRow map[string]interface{}

// NewService is used to generate a Google Spreadsheets API service
func NewService(keyPath string) (*sheets.Service, error) {
	b, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to read key file: %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("Unable to parse key file to config: %v", err)
	}

	client := config.Client(oauth2.NoContext)

	svc, err := sheets.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Sheets client: %v", err)
	}

	return svc, nil
}

// MapFromSheet is used to pull all records from a Gsheet and return SheetRows
func MapFromSheet(svc *sheets.Service, sheetID, worksheet string) (*SheetRows, error) {
	resp, err := svc.Spreadsheets.Values.Get(sheetID, worksheet).Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve data from sheet: %v", err)
	}

	data := make(SheetRows, 0)

	if len(resp.Values) == 0 {
		return nil, errors.New("No data found.")
	}

	for rowNum, row := range resp.Values[1:] {
		n := make(SheetRow, 0)
		n["ID"] = rowNum
		for idx, col := range resp.Values[0] {
			colLower := strings.ToLower(col.(string))
			if idx < len(row) {
				n[colLower] = row[idx]
			}
		}
		data = append(data, n)
	}

	return &data, nil
}

// GetHeadersFromSheet will return the current headers present in a sheet
func GetHeadersFromSheet(svc *sheets.Service, sheetID, worksheet string) ([]string, error) {
	resp, err := svc.Spreadsheets.Values.Get(sheetID, worksheet+"!1:1").Do()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve data from sheet: %v", err)
	}

	data := make([]string, 0)

	if len(resp.Values) == 0 {
		return nil, errors.New("No data found.")
	}

	for _, col := range resp.Values[0] {
		data = append(data, col.(string))
	}

	return data, nil
}

// GetWorksheetID will get the gid= value from a worksheet
func GetWorksheetID(svc *sheets.Service, sheetID, worksheet string) (int64, error) {
	gid, err := svc.Spreadsheets.Get(sheetID).Ranges(worksheet).Do()
	if err != nil {
		return -1, err
	}
	return gid.Sheets[0].Properties.SheetId, nil
}

// GetWorksheetTotalRows will return the total rows worksheet
func GetWorksheetTotalRows(svc *sheets.Service, sheetID, worksheet string) (int64, error) {
	gid, err := svc.Spreadsheets.Get(sheetID).Ranges(worksheet).Do()
	if err != nil {
		return -1, err
	}
	return gid.Sheets[0].Properties.GridProperties.RowCount, nil
}

// MakeWorksheet will check if a worksheet exists or make one
func MakeWorksheet(svc *sheets.Service, sheetID, worksheet string) error {
	req := sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Title: worksheet,
				GridProperties: &sheets.GridProperties{
					RowCount:    2,
					ColumnCount: 1,
				},
			},
		},
	}

	br := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err := svc.Spreadsheets.BatchUpdate(sheetID, br).Context(context.Background()).Do()
	if err != nil {
		return err
	}
	return nil
}

// SetHeaders will set the supplied headers as the first row in sheet
func SetHeaders(svc *sheets.Service, sheetID, worksheet string, headers []string) error {
	valRange := &sheets.ValueRange{}
	valRange.MajorDimension = "COLUMNS"

	if headers[0] != "TIMESTAMP" {
		headers = append([]string{"TIMESTAMP"}, headers...)
	}

	for _, col := range headers {
		valRange.Values = append(valRange.Values, []interface{}{col})
	}

	_, err := svc.Spreadsheets.Values.Update(sheetID, worksheet+"!A1", valRange).ValueInputOption("RAW").Do()
	if err != nil {
		return err
	}

	return nil
}

// AddLatestRow will add a row just after the header displaying the latest results
func AddLatestRow(svc *sheets.Service, sheetID, worksheet string) error {
	valRange := &sheets.ValueRange{}
	valRange.MajorDimension = "COLUMNS"

	curr, _ := GetHeadersFromSheet(svc, sheetID, worksheet)

	latestRow := make([]string, 0)
	for idx, _ := range curr {
		latestRow = append(latestRow, fmt.Sprintf("=index(OFFSET(A:A,0,%d),max(row(OFFSET(A3:A,0,%d))*(OFFSET(A3:A,0,%d)<>\"\")))", idx, idx, idx))
	}

	for _, col := range latestRow {
		valRange.Values = append(valRange.Values, []interface{}{col})
	}

	_, err := svc.Spreadsheets.Values.Update(sheetID, worksheet+"!A2", valRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return err
	}

	return nil
}

// AddRow will add a slice of strings as a new row
func AddRow(svc *sheets.Service, sheetID, worksheet string, row []interface{}) error {
	valRange := &sheets.ValueRange{}
	valRange.MajorDimension = "COLUMNS"

	for _, cell := range row {
		valRange.Values = append(valRange.Values, []interface{}{cell})
	}

	_, err := svc.Spreadsheets.Values.Append(sheetID, worksheet, valRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return err
	}

	return nil
}

// DeleteLastRows will delete the oldest X rows based on count value
func DeleteLastRows(svc *sheets.Service, sheetID string, worksheetID int64, count int64) error {
	req := sheets.Request{
		// DeleteRange: &sheets.DeleteRangeRequest{
		// 	Range: &sheets.GridRange{
		// 		SheetId:       worksheetID,
		// 		StartRowIndex: 2,
		// 		EndRowIndex:   2 + count,
		// 	},
		// 	ShiftDimension: "ROWS",
		// },
		DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId:    worksheetID,
				StartIndex: 2,
				EndIndex:   2 + count,
				Dimension:  "ROWS",
			},
		},
	}

	br := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err := svc.Spreadsheets.BatchUpdate(sheetID, br).Context(context.Background()).Do()
	if err != nil {
		return err
	}
	return nil
}
