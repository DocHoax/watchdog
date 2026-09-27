package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"time"
)

// RecurrenceConfig defines parameters for recurring incident pattern detection.
type RecurrenceConfig struct {
	MinOccurrences int           // Minimum incident occurrences to classify as recurring (default: 3)
	MaxLookback    time.Duration // Maximum lookback window (default: 7*24h)
}

// DefaultRecurrenceConfig returns standard defaults for recurrence pattern detection.
func DefaultRecurrenceConfig() RecurrenceConfig {
	return RecurrenceConfig{
		MinOccurrences: 3,
		MaxLookback:    7 * 24 * time.Hour,
	}
}

// DetectRecurrencePatterns analyzes historical incidents to identify periodic or recurring operational patterns.
func DetectRecurrencePatterns(incidents []Incident, since time.Duration, cfg *RecurrenceConfig) []RecurrencePattern {
	if cfg == nil {
		c := DefaultRecurrenceConfig()
		cfg = &c
	}
	if since <= 0 {
		since = 24 * time.Hour
	}
	if since > cfg.MaxLookback {
		since = cfg.MaxLookback
	}

	cutoff := time.Now().UTC().Add(-since)

	// Filter incidents within the lookback window
	var windowIncidents []Incident
	for _, inc := range incidents {
		if inc.StartTime.After(cutoff) || inc.StartTime.Equal(cutoff) {
			windowIncidents = append(windowIncidents, inc)
		}
	}

	if len(windowIncidents) < cfg.MinOccurrences {
		return nil
	}

	// Group incidents by (NodeID, EventType) and fleet-wide by (EventType)
	type groupKey struct {
		scope     RecurrenceScope
		targetID  string
		eventType string
	}

	groups := make(map[groupKey][]Incident)

	for _, inc := range windowIncidents {
		for _, sym := range inc.PrimarySymptoms {
			// Fleet-wide group
			fleetKey := groupKey{
				scope:     RecurrenceScopeFleet,
				targetID:  "fleet",
				eventType: sym,
			}
			groups[fleetKey] = append(groups[fleetKey], inc)

			// Per-node groups
			for _, nodeID := range inc.AffectedNodes {
				nodeKey := groupKey{
					scope:     RecurrenceScopeNode,
					targetID:  nodeID,
					eventType: sym,
				}
				groups[nodeKey] = append(groups[nodeKey], inc)
			}
		}
	}

	var patterns []RecurrencePattern

	for k, group := range groups {
		if len(group) < cfg.MinOccurrences {
			continue
		}

		// Deduplicate incidents in group by ID
		seenIncidents := make(map[string]Incident)
		for _, inc := range group {
			seenIncidents[inc.ID] = inc
		}
		if len(seenIncidents) < cfg.MinOccurrences {
			continue
		}

		var sortedIncidents []Incident
		for _, inc := range seenIncidents {
			sortedIncidents = append(sortedIncidents, inc)
		}
		sort.Slice(sortedIncidents, func(i, j int) bool {
			return sortedIncidents[i].StartTime.Before(sortedIncidents[j].StartTime)
		})

		count := len(sortedIncidents)
		firstOcc := sortedIncidents[0].StartTime
		mostRecent := sortedIncidents[count-1].StartTime

		// Compute inter-arrival intervals
		intervals := make([]float64, 0, count-1)
		for i := 1; i < count; i++ {
			dt := sortedIncidents[i].StartTime.Sub(sortedIncidents[i-1].StartTime).Seconds()
			if dt > 0 {
				intervals = append(intervals, dt)
			}
		}

		if len(intervals) == 0 {
			continue
		}

		// Calculate mean interval
		var sum float64
		for _, dt := range intervals {
			sum += dt
		}
		meanSec := sum / float64(len(intervals))
		avgInterval := time.Duration(meanSec * float64(time.Second))

		// Calculate median interval
		sort.Float64s(intervals)
		var medianSec float64
		n := len(intervals)
		if n%2 == 1 {
			medianSec = intervals[n/2]
		} else {
			medianSec = (intervals[(n/2)-1] + intervals[n/2]) / 2.0
		}
		medianInterval := time.Duration(medianSec * float64(time.Second))

		// Calculate sample variance and standard deviation of intervals
		var sumSqDiff float64
		for _, dt := range intervals {
			diff := dt - meanSec
			sumSqDiff += diff * diff
		}

		var stdDevSec float64
		if len(intervals) > 1 {
			stdDevSec = math.Sqrt(sumSqDiff / float64(len(intervals)-1))
		} else {
			stdDevSec = 0.0
		}

		// Coefficient of Variation (CV = sigma / mu)
		var cv float64
		if meanSec > 0 {
			cv = stdDevSec / meanSec
		}
		cv = sanitizeFloat(cv)

		// Determine confidence based on CV and sample count
		var confidence PredictionConfidence
		if count >= 5 && cv <= 0.40 {
			confidence = PredictionConfidenceHigh
		} else if count >= 3 && cv <= 0.80 {
			confidence = PredictionConfidenceMedium
		} else {
			confidence = PredictionConfidenceLow
		}

		relatedIDs := make([]string, 0, count)
		for _, inc := range sortedIncidents {
			relatedIDs = append(relatedIDs, inc.ID)
		}

		patternID := generateRecurrenceID(string(k.scope), k.targetID, k.eventType)
		summary := fmt.Sprintf("Event '%s' occurred %d times on %s '%s' (avg interval: %v, median: %v, regularity CV: %.2f)",
			k.eventType, count, k.scope, k.targetID, avgInterval.Round(time.Minute), medianInterval.Round(time.Minute), cv)

		patterns = append(patterns, RecurrencePattern{
			ID:                     patternID,
			Scope:                  k.scope,
			TargetID:               k.targetID,
			EventType:              k.eventType,
			OccurrenceCount:        count,
			FirstOccurrence:        firstOcc,
			MostRecentOccurrence:   mostRecent,
			AverageInterval:        avgInterval,
			MedianInterval:         medianInterval,
			CoefficientOfVariation: cv,
			Confidence:             confidence,
			RelatedIncidentIDs:     relatedIDs,
			Summary:                summary,
		})
	}

	// Sort patterns by occurrence count descending, then confidence
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].OccurrenceCount != patterns[j].OccurrenceCount {
			return patterns[i].OccurrenceCount > patterns[j].OccurrenceCount
		}
		return patterns[i].CoefficientOfVariation < patterns[j].CoefficientOfVariation
	})

	return patterns
}

func generateRecurrenceID(scope, targetID, eventType string) string {
	raw := fmt.Sprintf("%s:%s:%s", scope, targetID, eventType)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("rec-%s-%s", scope, hex.EncodeToString(hash[:4]))
}
