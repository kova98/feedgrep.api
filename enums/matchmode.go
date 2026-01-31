package enums

type MatchMode string

const (
	MatchModeInvalid MatchMode = ""

	// MatchModeBroad allows partial matches within words.
	// For example, the keyword "cat" will match "cat", "catalog", and "concatenate".
	MatchModeBroad MatchMode = "broad"

	// MatchModeExact requires an exact match of the whole word.
	// For example, the keyword "cat" will match "cat" but not "catalog" or "concatenate".
	MatchModeExact MatchMode = "exact"
)
