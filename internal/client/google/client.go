package google

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"os"
)

type Client struct {
	Service       *sheets.Service
	SpreadsheetID string
}

func NewGoogleClient(serviceAccountPath, spreadsheetID string) (*Client, error) {
	b, err := os.ReadFile(serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read service account file: %w", err)
	}

	config, err := google.JWTConfigFromJSON(b, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service account file: %w", err)
	}

	srv, err := sheets.NewService(context.Background(), option.WithHTTPClient(config.Client(context.Background())))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %w", err)
	}

	return &Client{
		Service:       srv,
		SpreadsheetID: spreadsheetID,
	}, nil
}
