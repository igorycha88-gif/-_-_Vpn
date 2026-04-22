package models

type DNSSettings struct {
	ID              int    `json:"id"`
	UpstreamRU      string `json:"upstream_ru"`
	UpstreamForeign  string `json:"upstream_foreign"`
	BlockAds        bool   `json:"block_ads"`
}

type DNSSettingsUpdateRequest struct {
	UpstreamRU      *string `json:"upstream_ru,omitempty"`
	UpstreamForeign  *string `json:"upstream_foreign,omitempty"`
	BlockAds        *bool   `json:"block_ads,omitempty"`
}

type DNSPreset struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Servers string `json:"servers"`
}
