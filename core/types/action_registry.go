package types

// ActionInfo contains metadata about an attestation action
type ActionInfo struct {
	ID          uint16 // Unique action ID (1-9 currently defined)
	Name        string // Action name as used in query_components
	IsBinary    bool   // True for binary (TRUE/FALSE) actions, false for numeric actions
	Description string // Human-readable description
}

// ActionRegistry maps action names to their metadata
// These correspond to the actions defined in the node's migrations:
// - 000-create-tn.sql (IDs 1-5: numeric actions)
// - 040-binary-attestation-actions.sql (IDs 6-9: binary actions)
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

// IsBinaryAction returns true if the action name corresponds to a binary action (IDs 6-9)
func IsBinaryAction(name string) bool {
	if info, ok := ActionRegistry[name]; ok {
		return info.IsBinary
	}
	return false
}

// IsBinaryActionID returns true if the action ID corresponds to a binary action (6-9)
func IsBinaryActionID(id uint16) bool {
	if info, ok := ActionByID[id]; ok {
		return info.IsBinary
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
