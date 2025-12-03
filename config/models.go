package config

type Config struct {
	Shops   []Shop
	Urls    Url
	SheetId string
}
type Shop struct {
	Name         string
	ClientId     string
	ClientSecret string
	UserId       int
	SheetRange   string
	Snapshots    []SnapshotTime
}

type Url struct {
	TokenUrl   string
	MetricsUrl string
}

type SnapshotTime struct {
	Time  string
	Range string
}
