package external

import (
	"encoding/json"
	"time"

	"gopkg.in/guregu/null.v3"
)

//
// Operation
//

type OperationLocation struct {
	OperationID uint64  `json:"-" gorm:"primaryKey"`
	Latitude    float64 `json:"latitude" gorm:"primaryKey"`
	Longitude   float64 `json:"longitude" gorm:"primaryKey"`
}

type OperationTime time.Time

func (t *OperationTime) UnmarshalJSON(data []byte) error {
	var s struct {
		Milliseconds int64 `json:"milliseconds"`
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	value := time.Unix(s.Milliseconds/1e3, 0).In(MoscowLocation)
	*t = OperationTime(value)
	return nil
}

type OperationAmount struct {
	Currency struct {
		Name string `json:"name" gorm:"column:currency;type:char(3);not null"`
	} `json:"currency" gorm:"embedded"`
	Value float64 `json:"value" gorm:"column:amount;not null"`
}

type OperationMerchant struct {
	Name   null.String `json:"name"`
	Region struct {
		Country null.String `json:"country" gorm:"type:char(3)"`
		City    null.String `json:"city"`
		Address null.String `json:"address"`
		ZIP     null.String `json:"zip" gorm:"column:zip"`
	} `json:"region" gorm:"embedded"`
}

type OperationLoyaltyBonus struct {
	OperationID uint64 `json:"-" gorm:"primaryKey"`
	Type        string `json:"loyaltyType" gorm:"primaryKey"`
	Amount      struct {
		ProgramID string  `json:"loyaltyProgramId" gorm:"primaryKey"`
		Value     float64 `json:"value" gorm:"not null"`
	} `json:"amount" gorm:"embedded"`
}

type Operation struct {
	ID uint64 `json:"id,string" gorm:"primaryKey;autoIncrement:false"`
	// nolint
	AuthorizationID  null.Int                `json:"authorizationId,string"`
	Time             OperationTime           `json:"operationTime" gorm:"type:timestamptz;not null;index;time"`
	DebitingTime     *OperationTime          `json:"debitingTime" gorm:"type:date"`
	Type             string                  `json:"type" gorm:"not null"`
	Group            string                  `json:"group" gorm:"not null"`
	Status           string                  `json:"status" gorm:"not null"`
	Description      string                  `json:"description" gorm:"not null"`
	Amount           OperationAmount         `json:"amount" gorm:"embedded"`
	AccountAmount    OperationAmount         `json:"accountAmount" gorm:"embedded;embeddedPrefix:account_"`
	Cashback         OperationAmount         `json:"cashbackAmount" gorm:"embedded;embeddedPrefix:cashback_"`
	LoyaltyBonus     []OperationLoyaltyBonus `json:"loyaltyBonus" gorm:"constraint:OnDelete:CASCADE"`
	SpendingCategory struct {
		Name string `json:"name" gorm:"column:category"`
	} `json:"spendingCategory" gorm:"embedded"`

	CardNumber  null.String `json:"cardNumber"`
	MCC         string      `json:"mccString" gorm:"type:char(4);not null"`
	CardPresent bool        `json:"cardPresent" gorm:"not null"`
	//Locations   []OperationLocation `json:"locations" gorm:"constraint:OnDelete:CASCADE;foreignKey:OperationID"`
	Merchant OperationMerchant `json:"merchant" gorm:"embedded;embeddedPrefix:merchant_"`

	AccountID string `json:"account" gorm:"tenant;constraint:OnDelete:CASCADE"`
	Account   Account

	HasShoppingReceipt bool `json:"hasShoppingReceipt" gorm:"-"`

	ShoppingReceipt *ShoppingReceipt `json:"-" gorm:"constraint:OnDelete:CASCADE"`
}

//
// Trading
//

type TradingOperationTime time.Time

func (t *TradingOperationTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, err := time.ParseInLocation("2006-01-02T15:04:05-07:00", s, MoscowLocation)
	if err != nil {
		return err
	}

	*t = TradingOperationTime(parsed)
	return nil
}

type TradingOperation struct {
	Username           string               `json:"-" gorm:"not null;index;tenant"`
	ID                 uint64               `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Time               TradingOperationTime `json:"date" gorm:"type:timestamptz;not null;time"`
	Type               string               `json:"operationType"`
	IsMarginCall       bool                 `json:"isMarginCall"`
	Issuer             null.String          `json:"issuer"`
	InstrumentType     null.String          `json:"instrumentType"`
	Ticker             null.String          `json:"ticker" gorm:"index"`
	Price              null.Float           `json:"price"`
	Payment            float64              `json:"payment" gorm:"not null"`
	Commission         null.Float           `json:"commission"`
	Currency           string               `json:"currency" gorm:"type:char(3);not null"`
	Quantity           null.Int             `json:"quantity"`
	CommissionCurrency null.String          `json:"commissionCurrency"`
	Description        string               `json:"description" gorm:"not null"`
}

type PurchasedSecurity struct {
	Ticker       string `json:"ticker" gorm:"primaryKey"`
	SecurityType string `json:"securityType" gorm:"primaryKey"`
	CurrentPrice struct {
		Currency string  `json:"currency" gorm:"type:char(3);not null"`
		Value    float64 `json:"value" gorm:"price;not null"`
	} `json:"currentPrice" gorm:"embedded"`

	Time time.Time `json:"-" gorm:"primaryKey;type:date"`
}

type CandleDate time.Time

func (d *CandleDate) UnmarshalJSON(data []byte) error {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*d = CandleDate(time.Unix(value, 0))
	return nil
}

type Candle struct {
	Ticker string     `json:"-" gorm:"primaryKey"`
	Date   CandleDate `json:"date" gorm:"primaryKey;type:date"`
	Open   float64    `json:"o" gorm:"not null"`
	Close  float64    `json:"c" gorm:"not null"`
	High   float64    `json:"h" gorm:"not null"`
	Low    float64    `json:"l" gorm:"not null"`
	Volume float64    `json:"v" gorm:"not null"`
}

//
// Shopping receipt
//

type ShoppingReceiptItem struct {
	ShoppingReceiptID uint64  `json:"-" gorm:"primaryKey"`
	Name              string  `json:"name" gorm:"primaryKey"`
	Price             float64 `json:"price" gorm:"primaryKey;not null"`
	Sum               float64 `json:"sum" gorm:"not null"`
	CategoryID        uint    `json:"cat_id" gorm:"not null"`
	Quantity          float64 `json:"quantity" gorm:"not null"`
	GoodID            uint    `json:"good_id" gorm:"not null"`
}

type ShoppingReceipt struct {
	OperationID uint64 `json:"-" gorm:"primaryKey"`
	Receipt     struct {
		OperationType      uint8                 `json:"operationType" gorm:"not null"`
		TotalSum           float64               `json:"totalSum" gorm:"not null"`
		Operator           string                `json:"operator" gorm:"not null"`
		User               string                `json:"user" gorm:"not null"`
		RetailPlaceAddress string                `json:"retailPlaceAddress" gorm:"not null"`
		Items              []ShoppingReceiptItem `json:"items"`
	} `json:"receipt" gorm:"embedded"`
}

//
// Account
//

type Account struct {
	ID   string `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"not null"`
	Type string `json:"accountType" gorm:"not null"`
}
