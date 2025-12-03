package avito

type AvitoTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type AvitoMetricsRequest struct {
	DateFrom string   `json:"dateFrom"`
	DateTo   string   `json:"dateTo"`
	Grouping string   `json:"grouping"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
	Metrics  []string `json:"metrics"`
}

type AvitoMetricsResponse struct {
	Result AvitoMetricsResult `json:"result"`
}

type AvitoMetricsResult struct {
	DataTotalCount int             `json:"dataTotalCount"`
	Groupings      []AvitoGrouping `json:"groupings"`
	Timestamp      string          `json:"timestamp"`
}

type AvitoGrouping struct {
	ID      int           `json:"id"`
	Metrics []AvitoMetric `json:"metrics"`
	Type    string        `json:"type"`
}

type AvitoMetric struct {
	Slug  string `json:"slug"`
	Value int    `json:"value"`
}

type ItemMetrics struct {
	ID                     int64
	Link                   string
	Title                  string
	Impressions            int
	Views                  int
	Contacts               int
	Spending               int
	BidPenny               int
	CostPerContactLastHour float64
}
