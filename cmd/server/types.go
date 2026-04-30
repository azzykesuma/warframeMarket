package main

type WfmEnvelope[T any] struct {
	ApiVersion string  `json:"apiVersion"`
	Data       T       `json:"data"`
	Error      *string `json:"error"`
}

type WfmItem struct {
	Id         string   `json:"id"`
	Slug       string   `json:"slug"`
	GameRef    string   `json:"gameRef"`
	Tags       []string `json:"tags"`
	Subtypes   []string `json:"subtypes"`
	Vaulted    bool     `json:"vaulted"`
	TradingTax int32    `json:"tradingTax"`
	Tradable   bool     `json:"tradable"`
	MaxRank    int32    `json:"maxRank"`
	Ducats     int32    `json:"ducats"`
	I18n       WfmI18n  `json:"i18n"`
}

type WfmI18n struct {
	En WfmLangDetail `json:"en"`
}

type WfmLangDetail struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Thumb       string `json:"thumb"`
	SubIcon     string `json:"subIcon"`
}

type WfmOrderListing struct {
	Order WfmOrder `json:"order"`
	User  WfmUser  `json:"user"`

	Id        string `json:"id"`
	ItemId    string `json:"itemId"`
	ItemSlug  string `json:"itemSlug"`
	Type      string `json:"type"`
	OrderType string `json:"order_type"`
	Platinum  int32  `json:"platinum"`
	Quantity  int32  `json:"quantity"`
	Subtype   string `json:"subtype"`
	Visible   bool   `json:"visible"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type WfmOrder struct {
	Id        string `json:"id"`
	ItemId    string `json:"itemId"`
	ItemSlug  string `json:"itemSlug"`
	Type      string `json:"type"`
	OrderType string `json:"order_type"`
	Platinum  int32  `json:"platinum"`
	Quantity  int32  `json:"quantity"`
	Subtype   string `json:"subtype"`
	Visible   bool   `json:"visible"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type WfmUser struct {
	Id         string `json:"id"`
	IngameName string `json:"ingameName"`
	Status     string `json:"status"`
	Platform   string `json:"platform"`
	Reputation int32  `json:"reputation"`
}

type WfmTopOrdersData struct {
	Sell []WfmOrderListing `json:"sell"`
	Buy  []WfmOrderListing `json:"buy"`
}

type WfmTransaction struct {
	Id        string `json:"id"`
	ItemName  string `json:"item_name"`
	Platinum  int32  `json:"platinum"`
	Quantity  int32  `json:"quantity"`
	OrderType string `json:"order_type"`
	UserName  string `json:"user_name"`
	CreatedAt string `json:"created_at"`
}
