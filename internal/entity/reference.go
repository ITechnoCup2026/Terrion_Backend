package entity

import "time"

type Commodity struct {
	ID        string `gorm:"column:id;primaryKey"`
	Slug      string `gorm:"column:slug"`
	Name      string `gorm:"column:name"`
	SpriteRow int    `gorm:"column:sprite_row"`
}

func (Commodity) TableName() string { return "commodity" }

type Variety struct {
	ID               string  `gorm:"column:id;primaryKey"`
	CommodityID      string  `gorm:"column:commodity_id"`
	Name             string  `gorm:"column:name"`
	GddRequirement   float64 `gorm:"column:gdd_requirement"`
	BaseTempC        float64 `gorm:"column:base_temp_c"`
	DaysToHarvestMin int     `gorm:"column:days_to_harvest_min"`
	DaysToHarvestMax int     `gorm:"column:days_to_harvest_max"`
	YieldPerHaMin    float64 `gorm:"column:yield_per_ha_min"`
	YieldPerHaMax    float64 `gorm:"column:yield_per_ha_max"`
}

func (Variety) TableName() string { return "variety" }

type FertiliserRate struct {
	CommodityID string  `gorm:"column:commodity_id;primaryKey"`
	InputItem   string  `gorm:"column:input_item;primaryKey"`
	KgPerHa     float64 `gorm:"column:kg_per_ha"`
	Source      string  `gorm:"column:source"`
}

func (FertiliserRate) TableName() string { return "fertiliser_rate" }

type ReferencePrice struct {
	CommodityID string    `gorm:"column:commodity_id;primaryKey"`
	Province    string    `gorm:"column:province;primaryKey"`
	WeekStart   time.Time `gorm:"column:week_start;primaryKey"`
	PricePerKg  float64   `gorm:"column:price_per_kg"`
	Source      string    `gorm:"column:source"`
}

func (ReferencePrice) TableName() string { return "reference_price" }

type RegionStat struct {
	RegionCode       string  `gorm:"column:region_code;primaryKey"`
	RegionName       string  `gorm:"column:region_name"`
	Level            string  `gorm:"column:level;type:region_level"`
	CommodityID      string  `gorm:"column:commodity_id;primaryKey"`
	Year             int     `gorm:"column:year;primaryKey"`
	ProductionTonnes float64 `gorm:"column:production_tonnes"`
	HarvestedAreaHa  float64 `gorm:"column:harvested_area_ha"`
	Source           string  `gorm:"column:source"`
}

func (RegionStat) TableName() string { return "region_stat" }
