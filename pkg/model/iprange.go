package model

import (
	"fmt"
	"net"
	"strings"
)

// IPRange represents an IP range (CIDR)
type IPRange struct {
	CIDR   string   // CIDR notation (e.g., "192.168.1.0/24")
	Labels []string // Labels/tags associated with this range
}

// IPRangeSet is a set of IP ranges
type IPRangeSet struct {
	Ranges []IPRange
}

// NewIPRangeSet creates a new empty set of IP ranges
func NewIPRangeSet() *IPRangeSet {
	return &IPRangeSet{
		Ranges: []IPRange{},
	}
}

// Add adds an IP range to the set
func (s *IPRangeSet) Add(cidr string, labels []string) error {
	// Validate CIDR format
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format '%s': %w", cidr, err)
	}

	s.Ranges = append(s.Ranges, IPRange{
		CIDR:   cidr,
		Labels: labels,
	})
	return nil
}

// AddIPRange adds an IPRange object to the set
func (s *IPRangeSet) AddIPRange(ipRange IPRange) error {
	return s.Add(ipRange.CIDR, ipRange.Labels)
}

// Filter returns a new IPRangeSet with only ranges that have the specified labels
func (s *IPRangeSet) Filter(includeLabels []string, excludeLabels []string) *IPRangeSet {
	result := NewIPRangeSet()

	for _, ipRange := range s.Ranges {
		// If include labels are specified, check if any of them match
		include := len(includeLabels) == 0
		for _, includeLabel := range includeLabels {
			for _, label := range ipRange.Labels {
				if includeLabel == label {
					include = true
					break
				}
			}
			if include {
				break
			}
		}

		// If exclude labels are specified, check if any of them match
		for _, excludeLabel := range excludeLabels {
			for _, label := range ipRange.Labels {
				if excludeLabel == label {
					include = false
					break
				}
			}
			if !include {
				break
			}
		}

		if include {
			result.Ranges = append(result.Ranges, ipRange)
		}
	}

	return result
}

// Merge combines this IPRangeSet with another, returning a new set
func (s *IPRangeSet) Merge(other *IPRangeSet) *IPRangeSet {
	result := NewIPRangeSet()
	result.Ranges = append(result.Ranges, s.Ranges...)
	result.Ranges = append(result.Ranges, other.Ranges...)
	return result
}

// Diff compares this IPRangeSet with another, returning added and removed ranges
func (s *IPRangeSet) Diff(other *IPRangeSet) (added, removed *IPRangeSet) {
	added = NewIPRangeSet()
	removed = NewIPRangeSet()

	// Find ranges in other that are not in this set (added)
	otherMap := make(map[string]IPRange)
	for _, ipRange := range other.Ranges {
		otherMap[ipRange.CIDR] = ipRange
	}

	thisMap := make(map[string]IPRange)
	for _, ipRange := range s.Ranges {
		thisMap[ipRange.CIDR] = ipRange
	}

	for cidr, ipRange := range otherMap {
		if _, exists := thisMap[cidr]; !exists {
			added.Ranges = append(added.Ranges, ipRange)
		}
	}

	// Find ranges in this set that are not in other (removed)
	for cidr, ipRange := range thisMap {
		if _, exists := otherMap[cidr]; !exists {
			removed.Ranges = append(removed.Ranges, ipRange)
		}
	}

	return added, removed
}

// String returns a string representation of the IP range set
func (s *IPRangeSet) String() string {
	cidrs := make([]string, len(s.Ranges))
	for i, ipRange := range s.Ranges {
		cidrs[i] = ipRange.CIDR
	}
	return strings.Join(cidrs, ", ")
}

// Count returns the number of IP ranges in the set
func (s *IPRangeSet) Count() int {
	return len(s.Ranges)
}

// GetCIDRs returns a slice of all CIDRs in the set
func (s *IPRangeSet) GetCIDRs() []string {
	cidrs := make([]string, len(s.Ranges))
	for i, ipRange := range s.Ranges {
		cidrs[i] = ipRange.CIDR
	}
	return cidrs
}
