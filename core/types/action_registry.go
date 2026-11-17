package types

// ActionInfo contains metadata about an attestation action
type ActionInfo struct {
	ID          uint16 // Unique action ID (see ActionRegistry for the defined set)
	Name        string // Action name as used in query_components
	IsBinary    bool   // True for binary (TRUE/FALSE) actions, false for numeric actions
	IsOutcome   bool   `json:"is_outcome"` // True for outcome actions (YES/NO/INVALID), ids 13-16
	Description string // Human-readable description
}

// ActionRegistry maps action names to their metadata
// These correspond to the actions defined in the node's migrations:
// - 000-create-tn.sql (IDs 1-5: numeric actions)
// - 040-binary-attestation-actions.sql (IDs 6-9: binary actions)
// - 055-index-change-attestation-action.sql (ID 12: binary action)
// - 063-outcome-attestation-actions.sql (IDs 13-16: outcome actions)
//
// IDs are not contiguous, and binary IDs are not a range: 10 and 11 exist on the
// node as numeric actions and are deliberately absent here. Anything deciding
// whether an action is binary must ask this registry rather than compare against
// a bound.
//
// The outcome actions (13-16) are not binary. Each answers 1 (YES), 0 (NO) or
// -1 (INVALID) as a numeric row, so ParseBooleanResult refuses them and
// ParseOutcomeResult reads them.
var ActionRegistry = map[string]ActionInfo{
	// Numeric actions (IDs 1-5) - return uint256[], int256[]
	"get_record": {
		ID:          1,
		Name:        "get_record",
		IsBinary:    false,
		Description: "Get record value at a specific timestamp",
	},
	"get_index": {
		ID:          2,
		Name:        "get_index",
		IsBinary:    false,
		Description: "Get index value at a specific timestamp",
	},
	"get_change_over_time": {
		ID:          3,
		Name:        "get_change_over_time",
		IsBinary:    false,
		Description: "Get change in value over a time period",
	},
	"get_last_record": {
		ID:          4,
		Name:        "get_last_record",
		IsBinary:    false,
		Description: "Get the most recent record value",
	},
	"get_first_record": {
		ID:          5,
		Name:        "get_first_record",
		IsBinary:    false,
		Description: "Get the earliest record value",
	},

	// Binary actions (IDs 6-9) - return bool (TRUE/FALSE)
	"price_above_threshold": {
		ID:          6,
		Name:        "price_above_threshold",
		IsBinary:    true,
		Description: "TRUE if value > threshold (e.g., 'Will BTC exceed $100k?')",
	},
	"price_below_threshold": {
		ID:          7,
		Name:        "price_below_threshold",
		IsBinary:    true,
		Description: "TRUE if value < threshold (e.g., 'Will unemployment drop below 4%?')",
	},
	"value_in_range": {
		ID:          8,
		Name:        "value_in_range",
		IsBinary:    true,
		Description: "TRUE if min <= value <= max (e.g., 'Will BTC stay between $90k-$110k?')",
	},
	"value_equals": {
		ID:          9,
		Name:        "value_equals",
		IsBinary:    true,
		Description: "TRUE if value = target ± tolerance (e.g., 'Will Fed rate be exactly 5.25%?')",
	},

	// Binary action (ID 12) - returns bool (TRUE/FALSE)
	"index_change_in_range": {
		ID:          12,
		Name:        "index_change_in_range",
		IsBinary:    true,
		Description: "TRUE if min <= percentage change < max (e.g., 'Will inflation land between 2% and 3%?')",
	},

	// Outcome actions (IDs 13-16) - return 1 (YES), 0 (NO) or -1 (INVALID)
	"index_change_in_range_v2": {
		ID:          13,
		Name:        "index_change_in_range_v2",
		IsBinary:    false,
		IsOutcome:   true,
		Description: "YES if min <= percentage change < max, INVALID if the print never arrives",
	},
	"price_above_threshold_v2": {
		ID:          14,
		Name:        "price_above_threshold_v2",
		IsBinary:    false,
		IsOutcome:   true,
		Description: "YES if value > threshold, INVALID if the print never arrives",
	},
	"price_below_threshold_v2": {
		ID:          15,
		Name:        "price_below_threshold_v2",
		IsBinary:    false,
		IsOutcome:   true,
		Description: "YES if value < threshold, INVALID if the print never arrives",
	},
	"value_in_range_v2": {
		ID:          16,
		Name:        "value_in_range_v2",
		IsBinary:    false,
		IsOutcome:   true,
		Description: "YES if min <= value < max, INVALID if the print never arrives",
	},
}

// ActionByID maps action IDs to their metadata
var ActionByID = map[uint16]ActionInfo{}

func init() {
	for _, info := range ActionRegistry {
		ActionByID[info.ID] = info
	}
}

// GetActionInfo returns the ActionInfo for a given action name, or nil if not found
func GetActionInfo(name string) *ActionInfo {
	if info, ok := ActionRegistry[name]; ok {
		return &info
	}
	return nil
}

// GetActionInfoByID returns the ActionInfo for a given action ID, or nil if not found
func GetActionInfoByID(id uint16) *ActionInfo {
	if info, ok := ActionByID[id]; ok {
		return &info
	}
	return nil
}

// IsBinaryAction returns true if the action name corresponds to a binary action
func IsBinaryAction(name string) bool {
	if info, ok := ActionRegistry[name]; ok {
		return info.IsBinary
	}
	return false
}

// IsBinaryActionID returns true if the action ID corresponds to a binary action
func IsBinaryActionID(id uint16) bool {
	if info, ok := ActionByID[id]; ok {
		return info.IsBinary
	}
	return false
}

// IsOutcomeAction returns true if the action name corresponds to an outcome
// action (YES, NO or INVALID)
func IsOutcomeAction(name string) bool {
	if info, ok := ActionRegistry[name]; ok {
		return info.IsOutcome
	}
	return false
}

// IsOutcomeActionID returns true if the action ID corresponds to an outcome
// action (YES, NO or INVALID)
func IsOutcomeActionID(id uint16) bool {
	if info, ok := ActionByID[id]; ok {
		return info.IsOutcome
	}
	return false
}

// GetActionID returns the action ID for a given action name, or 0 if not found
func GetActionID(name string) uint16 {
	if info, ok := ActionRegistry[name]; ok {
		return info.ID
	}
	return 0
}

// GetActionName returns the action name for a given action ID, or empty string if not found
func GetActionName(id uint16) string {
	if info, ok := ActionByID[id]; ok {
		return info.Name
	}
	return ""
}

// ValidateActionName returns an error if the action name is not recognized
func ValidateActionName(name string) error {
	if _, ok := ActionRegistry[name]; !ok {
		return &UnknownActionError{Name: name}
	}
	return nil
}

// UnknownActionError is returned when an unrecognized action name is used
type UnknownActionError struct {
	Name string
}

func (e *UnknownActionError) Error() string {
	return "unknown action: " + e.Name
}
