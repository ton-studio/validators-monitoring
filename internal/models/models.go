package models

import "time"

type Cycle struct {
	CycleID   int       `json:"cycle_id"`
	CycleInfo CycleInfo `json:"cycle_info"`
}

type CycleInfo struct {
	UtimeSince  int64       `json:"utime_since"`
	UtimeUntil  int64       `json:"utime_until"`
	TotalWeight int64       `json:"total_weight"`
	Validators  []Validator `json:"validators"`
}

type Validator struct {
	ADNLAddr      string      `json:"adnl_addr"`
	PubKey        string      `json:"pubkey"`
	Weight        int64       `json:"weight"`
	Index         int         `json:"index"`
	Stake         int64       `json:"stake"`
	MaxFactor     int         `json:"max_factor"`
	WalletAddress string      `json:"wallet_address"`
	Complaints    []Complaint `json:"complaints"`
}

type ScoreboardResponse struct {
	Scoreboard []CycleScoreboardRow `json:"scoreboard"`
}

type CycleScoreboardRow struct {
	CycleID       uint32  `json:"cycle_id"`
	UtimeSince    int64   `json:"utime_since"`
	UtimeUntil    int64   `json:"utime_until"`
	ADNLAddr      string  `json:"adnl_addr"`
	PubKey        string  `json:"pubkey"`
	PubKeyHash    string  `json:"pubkey_hash"`
	Weight        int64   `json:"weight"`
	Index         uint16  `json:"idx"`
	Stake         int64   `json:"stake"`
	ValidatorADNL string  `json:"validator_adnl"`
	Efficiency    float64 `json:"efficiency"`
}

type ValidatorStatusHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

type ValidatorEfficiency struct {
	ADNLAddr      string  `db:"adnl_addr"`
	ValidatorADNL string  `db:"validator_adnl"`
	IntervalStart uint32  `db:"interval_start"`
	Efficiency    float64 `db:"avg_efficiency"`
	CycleID       uint32  `db:""`
}

type ValidatorStatus string

const (
	StatusOK           ValidatorStatus = "ok"
	StatusNotOK        ValidatorStatus = "not ok"
	StatusAcknowledged ValidatorStatus = "acknowledged"
	StatusUnknown      ValidatorStatus = "unknown"
)

type EfficiencyDataResponse struct {
	Timestamp uint32  `json:"timestamp"`
	Value     float64 `json:"value"`
	CycleID   uint32  `json:"cycle_id"`
}

type Complaint struct {
	ElectionId        int           `json:"election_id"`
	Hash              string        `json:"hash"`
	Pubkey            string        `json:"pubkey"`
	AdnlAddr          string        `json:"adnl_addr"`
	Description       string        `json:"description"`
	CreatedTime       int           `json:"created_time"`
	Severity          int           `json:"severity"`
	RewardAddr        string        `json:"reward_addr"`
	Paid              int           `json:"paid"`
	SuggestedFine     int64         `json:"suggested_fine"`
	SuggestedFinePart int           `json:"suggested_fine_part"`
	VotedValidators   []interface{} `json:"voted_validators"`
	VsetId            string        `json:"vset_id"`
	WeightRemaining   float64       `json:"weight_remaining"`
	ApprovedPercent   float32       `json:"approved_percent"`
	IsPassed          bool          `json:"is_passed"`
	Pseudohash        string        `json:"pseudohash"`
	WalletAddress     string        `json:"wallet_address"`
}

type Meta map[string]struct {
	Weight        string  `json:"weight"`
	Index         uint16  `json:"index"`
	Stake         string  `json:"stake"`
	WalletAddress string  `json:"wallet_address"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	CycleID       uint32  `json:"cycle_id"`
}
