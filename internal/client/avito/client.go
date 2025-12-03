package avito

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"time"
)

type tokenCache struct {
	Token     string
	ExpiresAt time.Time
}

type AvitoClient struct {
	logger     *zap.Logger
	TokenUrl   string
	MetricsUrl string

	tokens map[string]tokenCache // ключ = client_id магазина
}

func NewAvitoClient(logger *zap.Logger, TokenUrl, MetricsUrl string) *AvitoClient {
	return &AvitoClient{
		logger:     logger,
		TokenUrl:   TokenUrl,
		MetricsUrl: MetricsUrl,
		tokens:     make(map[string]tokenCache),
	}
}

// Получение нового токена для конкретного магазина
func (a *AvitoClient) getToken(cId, cSec string) error {
	form := url.Values{}
	form.Add("grant_type", "client_credentials")
	form.Add("client_id", cId)
	form.Add("client_secret", cSec)

	req, err := http.NewRequest("POST", a.TokenUrl, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	var tr AvitoTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}

	// сохраняем токен *отдельно* для каждого client_id
	a.tokens[cId] = tokenCache{
		Token:     tr.AccessToken,
		ExpiresAt: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}

	return nil
}

// Возвращает токен для магазина, обновляет если просрочен
func (a *AvitoClient) Token(cId, cSec string) (string, error) {
	tc, ok := a.tokens[cId]

	// если нет токена или скоро протухнет — получаем новыйу
	if !ok || tc.Token == "" || time.Now().After(tc.ExpiresAt.Add(-5*time.Minute)) {
		if err := a.getToken(cId, cSec); err != nil {
			a.logger.Error("failed to refresh token", zap.Error(err))
			return "", err
		}

		tc = a.tokens[cId]
	}

	return tc.Token, nil
}

func (a *AvitoClient) GetAvitoMetrics(uId int, cId, cSec string) (AvitoMetricsData, error) {
	token, err := a.Token(cId, cSec)
	if err != nil {
		return AvitoMetricsData{}, err
	}

	metricsUrl := fmt.Sprintf("%s%d/items", a.MetricsUrl, uId)

	reqBody := AvitoMetricsRequest{
		DateFrom: time.Now().Format("2006-01-02"),
		DateTo:   time.Now().Format("2006-01-02"),
		Grouping: "totals",
		Limit:    1000,
		Offset:   0,
		Metrics:  []string{"views", "contacts", "impressions", "spending", "clickPackages", "impressionsToViewsConversion", "viewsToContactsConversion"},
	}

	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", metricsUrl, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.logger.Error(err.Error())
		return AvitoMetricsData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error(fmt.Sprintf("bad status: %s", resp.Status))
		return AvitoMetricsData{}, fmt.Errorf("bad status: %s", resp.Status)
	}

	var data AvitoMetricsResponse
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		a.logger.Error(err.Error())
		return AvitoMetricsData{}, err
	}

	metricsMap := make(map[string]int)
	for _, grouping := range data.Result.Groupings {
		for _, metric := range grouping.Metrics {
			metricsMap[metric.Slug] = metric.Value
		}
	}

	return AvitoMetricsData{
		Spending:                     metricsMap["spending"],
		Impressions:                  metricsMap["impressions"],
		Contacts:                     metricsMap["contacts"],
		Views:                        metricsMap["views"],
		ImpressionsToViewsConversion: metricsMap["impressionsToViewsConversion"],
		ViewsToContactsConversion:    metricsMap["viewsToContactsConversion"],
	}, nil
}

func (a *AvitoClient) GetMetricsForAllItems(uId int, cId, cSec string, logger *zap.Logger) ([]ItemMetrics, error) {
	logger.Info("Start GetMetricsForAllItems", zap.Int("userId", uId))

	token, err := a.Token(cId, cSec)
	if err != nil {
		logger.Error("Failed to get token", zap.Error(err))
		return nil, err
	}
	logger.Debug("Token retrieved", zap.String("token", token[:10]+"..."))

	// --- 1. Получение всех активных объявлений ---
	items := []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}{}
	page := 1
	for {
		url := fmt.Sprintf("https://api.avito.ru/core/v1/items?status=active&per_page=100&page=%d", page)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("Failed to get items page", zap.Int("page", page), zap.Error(err))
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Error("Bad status when fetching items", zap.Int("page", page), zap.String("status", resp.Status))
			return nil, fmt.Errorf("bad status: %s", resp.Status)
		}

		var res struct {
			Meta struct {
				Page    int `json:"page"`
				PerPage int `json:"per_page"`
			} `json:"meta"`
			Resources []struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"resources"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			logger.Error("Failed to decode items response", zap.Int("page", page), zap.Error(err))
			return nil, err
		}

		logger.Info("Items page fetched", zap.Int("page", page), zap.Int("count", len(res.Resources)))

		items = append(items, res.Resources...)
		if len(res.Resources) < 100 {
			break
		}
		page++
	}
	logger.Info("All active items fetched", zap.Int("totalItems", len(items)))

	if len(items) == 0 {
		logger.Warn("No items found")
		return nil, nil
	}

	// --- 2. Получаем метрики по объявлениям ---
	metricsURL := fmt.Sprintf("%s%d/items", a.MetricsUrl, uId)
	reqBody := AvitoMetricsRequest{
		DateFrom: time.Now().Format("2006-01-02"),
		DateTo:   time.Now().Format("2006-01-02"),
		Grouping: "item",
		Limit:    1000,
		Offset:   0,
		Metrics:  []string{"views", "contacts", "impressions", "spending"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", metricsURL, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("Failed to get item metrics", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Bad status fetching metrics", zap.String("status", resp.Status))
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var metricsRes AvitoMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&metricsRes); err != nil {
		logger.Error("Failed to decode metrics response", zap.Error(err))
		return nil, err
	}
	logger.Info("Item metrics fetched", zap.Int("metricsCount", len(metricsRes.Result.Groupings)))

	// --- 3. Получаем bidPenny батчами по 200 ---
	idToBid := make(map[int64]int)
	for i := 0; i < len(items); i += 200 {
		end := i + 200
		if end > len(items) {
			end = len(items)
		}
		batchIDs := []int64{}
		for _, it := range items[i:end] {
			batchIDs = append(batchIDs, it.ID)
		}

		batchBody, _ := json.Marshal(map[string][]int64{"itemIDs": batchIDs})
		req, _ := http.NewRequest("POST", "https://api.avito.ru/cpxpromo/1/getPromotionsByItemIds", bytes.NewBuffer(batchBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("Failed to get bid batch", zap.Int("batchStart", i), zap.Int("batchEnd", end), zap.Error(err))
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Error("Bad status fetching bid", zap.Int("batchStart", i), zap.Int("batchEnd", end), zap.String("status", resp.Status))
			return nil, fmt.Errorf("bad status: %s", resp.Status)
		}

		var bidRes struct {
			Items []struct {
				ItemID          int64 `json:"itemID"`
				ManualPromotion struct {
					BidPenny int `json:"bidPenny"`
				} `json:"manualPromotion"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&bidRes); err != nil {
			logger.Error("Failed to decode bid response", zap.Int("batchStart", i), zap.Int("batchEnd", end), zap.Error(err))
			return nil, err
		}

		logger.Debug("Bid batch decoded", zap.Int("batchStart", i), zap.Int("batchEnd", end), zap.Int("bidsCount", len(bidRes.Items)))

		for _, it := range bidRes.Items {
			idToBid[it.ItemID] = it.ManualPromotion.BidPenny
		}
	}
	logger.Info("All bids fetched", zap.Int("totalBids", len(idToBid)))

	// --- 4. Собираем финальные данные ---
	itemMetrics := []ItemMetrics{}
	for _, it := range items {
		var mAvito AvitoGrouping
		found := false
		for _, g := range metricsRes.Result.Groupings {
			if int64(g.ID) == it.ID {
				mAvito = g
				found = true
				break
			}
		}
		if !found {
			logger.Warn("Metrics not found for item", zap.Int64("itemID", it.ID))
		}

		metricsMap := map[string]int{}
		for _, metric := range mAvito.Metrics {
			metricsMap[metric.Slug] = metric.Value
		}

		contacts := metricsMap["contacts"]
		spending := metricsMap["spending"]

		cpc := 0.0
		if contacts > 0 {
			cpc = float64(spending) / float64(contacts)
		}

		itemMetrics = append(itemMetrics, ItemMetrics{
			ID:                     it.ID,
			Link:                   it.URL,
			Title:                  it.Title,
			Impressions:            metricsMap["impressions"],
			Views:                  metricsMap["views"],
			Contacts:               contacts,
			Spending:               spending,
			BidPenny:               idToBid[it.ID],
			CostPerContactLastHour: cpc,
		})
		logger.Debug("ItemMetrics prepared", zap.Int64("itemID", it.ID), zap.Any("metricsMap", metricsMap), zap.Int("bid", idToBid[it.ID]))
	}

	logger.Info("GetMetricsForAllItems finished", zap.Int("totalItems", len(itemMetrics)))
	return itemMetrics, nil
}
