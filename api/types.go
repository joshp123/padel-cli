package api

type Tenant struct {
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Address    Address    `json:"address"`
	Resources  []Resource `json:"resources"`
}

type Address struct {
	Street   string     `json:"street"`
	City     string     `json:"city"`
	Country  string     `json:"country"`
	Zip      string     `json:"postal_code"`
	Coord    Coordinate `json:"coordinate"`
	TimeZone string     `json:"timezone"`
}

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Resource struct {
	ResourceID string             `json:"resource_id"`
	Name       string             `json:"name"`
	Properties ResourceProperties `json:"properties"`
}

type ResourceProperties struct {
	ResourceType    string `json:"resource_type"`    // "indoor" or "outdoor"
	ResourceSize    string `json:"resource_size"`    // "single" or "double"
	ResourceFeature string `json:"resource_feature"` // "panoramic", etc.
}

func (r Resource) IsIndoor() bool {
	return r.Properties.ResourceType == "indoor"
}

type AvailabilityResource struct {
	ResourceID string `json:"resource_id"`
	StartDate  string `json:"start_date"`
	Slots      []Slot `json:"slots"`
}

type Slot struct {
	StartTime string `json:"start_time"`
	Duration  int    `json:"duration"`
	Price     string `json:"price"`
}

type Match struct {
	MatchID      string `json:"match_id"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Status       string `json:"status"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Price        string `json:"price"`
	CreatedAt    string `json:"created_at"`
	Tenant       Tenant `json:"tenant"`
}
