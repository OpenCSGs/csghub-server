package types

import (
	"math"
	"strconv"
	"strings"
)

// User tag vocabulary for upstream channel routing. Tags live in the
// user_tags relation (tag names in the tags table) and are resolved into the
// request context by the gateway's user-tags middleware. Operators assign
// them with plain SQL.
const (
	// UserTagLowPriority marks a low-priority user: they may only route to
	// upstreams of the low-priority channel (upstream priority 0).
	UserTagLowPriority = "low_priority"
	// UserTagPriorityPrefix starts a generic priority tag "priority:<n>"
	// (e.g. "priority:50") assigning the user the numeric channel priority n.
	// It supports intermediate tiers beyond the named low_priority tag.
	UserTagPriorityPrefix = "priority:"
	// UnrestrictedUpstreamPriority is the channel priority of a caller with
	// no priority tag: they may route to every upstream and prefer the
	// highest-priority channel available.
	UnrestrictedUpstreamPriority = math.MaxInt
)

// IsUserRoutingTagName reports whether a tag name belongs to the
// operator-managed upstream channel routing namespace (low_priority and
// priority:<n>). These tags decide the caller's upstream channel priority, so
// self-service user tag updates must neither add nor clear them.
//
// The match is exact on purpose: the SQL read/delete predicates compare the
// stored name verbatim, and tag names are stored as created. Trimming here
// would make Go and SQL disagree about a name like "low_priority " — hidden
// from the profile yet preserved by no self-service guard — so both sides must
// keep the same rule.
func IsUserRoutingTagName(name string) bool {
	return name == UserTagLowPriority || strings.HasPrefix(name, UserTagPriorityPrefix)
}

// UserRoutingPriorityFromTags resolves the caller's upstream channel
// priority from their user tags. Semantics:
//
//   - no recognized priority tag → UnrestrictedUpstreamPriority (the default,
//     non-low-priority user: prefers high-priority upstreams, may still fall
//     back to lower channels)
//   - "low_priority" → 0 (may only route to low-priority upstreams)
//   - "priority:<n>" → n (generic tier; negative or malformed values are
//     ignored)
//   - several priority tags → the highest wins
func UserRoutingPriorityFromTags(tags []string) int {
	priority := -1
	for _, tag := range tags {
		var n int
		switch {
		case strings.TrimSpace(tag) == UserTagLowPriority:
			n = 0
		case strings.HasPrefix(tag, UserTagPriorityPrefix):
			parsed, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(tag, UserTagPriorityPrefix)))
			if err != nil || parsed < 0 {
				continue
			}
			n = parsed
		default:
			continue
		}
		if n > priority {
			priority = n
		}
	}
	if priority < 0 {
		return UnrestrictedUpstreamPriority
	}
	return priority
}
