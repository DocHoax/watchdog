package incidents

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DocHoax/watchdog/internal/intelligence"
)

// PeriodicityClassification characterizes the timing pattern of recurring incidents.
type PeriodicityClassification string

const (
	PeriodicityPeriodic     PeriodicityClassification = "periodic"
	PeriodicitySemiPeriodic PeriodicityClassification = "semi_periodic"
	PeriodicityIrregular    PeriodicityClassification = "irregular"
	PeriodicityIsolated     PeriodicityClassification = "isolated"
)

// RecurrenceAnalysis details historical recurrence statistics for an incident pattern.
type RecurrenceAnalysis struct {
	PatternKey             string                    `json:"pattern_key"`
	OccurrenceCount        int                       `json:"occurrence_count"`
	FirstOccurrence        time.Time                 `json:"first_occurrence"`
	MostRecentOccurrence   time.Time                 `json:"most_recent_occurrence"`
	AverageInterval        time.Duration             `json:"average_interval"`
	MedianInterval         time.Duration             `json:"median_interval"`
	StandardDeviation      time.Duration             `json:"standard_deviation"`
	CoefficientOfVariation float64                   `json:"coefficient_of_variation"`
	Periodicity            PeriodicityClassification `json:"periodicity"`
	IsFlapping             bool                      `json:"is_flapping"`
	Confidence             string                    `json:"confidence"`
	HistoricalIncidentIDs  []string                  `json:"historical_incident_ids"`
	Summary                string                    `json:"summary"`
}

// IncidentInvestigationReport provides a consolidated, operator-friendly diagnostic dossier.
type IncidentInvestigationReport struct {
	Incident           Incident                           `json:"incident"`
	Recurrence         *RecurrenceAnalysis                `json:"recurrence,omitempty"`
	SimilarIncidents   []SimilarIncidentResult            `json:"similar_incidents,omitempty"`
	TimelineHighlights []IncidentTimelineEntry            `json:"timeline_highlights,omitempty"`
	Impact             ImpactAnalysis                     `json:"impact"`
	Findings           []intelligence.IntelligenceFinding `json:"findings"`
	Summary            string                             `json:"summary"`
	GeneratedAt        time.Time                          `json:"generated_at"`
}

// AnalyzeRecurrence calculates inter-arrival time statistics across similar historical incidents.
func AnalyzeRecurrence(target Incident, history []Incident) *RecurrenceAnalysis {
	// Find matches with similarity >= 0.40 or sharing the same primary symptoms
	var matching []Incident
	for _, h := range history {
		if h.ID == target.ID {
			matching = append(matching, h)
			continue
		}
		score, _, _ := CalculateSimilarity(target, h)
		if score >= 0.40 {
			matching = append(matching, h)
		}
	}

	if len(matching) <= 1 {
		return &RecurrenceAnalysis{
			PatternKey:            target.Title,
			OccurrenceCount:       len(matching),
			FirstOccurrence:       target.StartTime,
			MostRecentOccurrence:  target.StartTime,
			Periodicity:           PeriodicityIsolated,
			IsFlapping:            false,
			Confidence:            "high",
			HistoricalIncidentIDs: []string{target.ID},
			Summary:               "Isolated incident; no prior recurring patterns detected across historical lookback.",
		}
	}

	// Sort chronologically ascending
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].StartTime.Before(matching[j].StartTime)
	})

	var incidentIDs []string
	var timestamps []time.Time
	for _, m := range matching {
		incidentIDs = append(incidentIDs, m.ID)
		timestamps = append(timestamps, m.StartTime)
	}

	// Compute inter-arrival intervals
	var intervals []float64 // in seconds
	for i := 1; i < len(timestamps); i++ {
		diffSec := timestamps[i].Sub(timestamps[i-1]).Seconds()
		if diffSec > 0 {
			intervals = append(intervals, diffSec)
		}
	}

	if len(intervals) == 0 {
		return &RecurrenceAnalysis{
			PatternKey:            target.Title,
			OccurrenceCount:       len(matching),
			FirstOccurrence:       timestamps[0],
			MostRecentOccurrence:  timestamps[len(timestamps)-1],
			Periodicity:           PeriodicityIsolated,
			IsFlapping:            false,
			Confidence:            "medium",
			HistoricalIncidentIDs: incidentIDs,
			Summary:               fmt.Sprintf("Observed %d instances with zero inter-arrival duration.", len(matching)),
		}
	}

	// Calculate Mean
	var sum float64
	for _, val := range intervals {
		sum += val
	}
	meanSec := sum / float64(len(intervals))

	// Calculate Median
	sortedIntervals := make([]float64, len(intervals))
	copy(sortedIntervals, intervals)
	sort.Float64s(sortedIntervals)

	var medianSec float64
	mid := len(sortedIntervals) / 2
	if len(sortedIntervals)%2 == 0 {
		medianSec = (sortedIntervals[mid-1] + sortedIntervals[mid]) / 2.0
	} else {
		medianSec = sortedIntervals[mid]
	}

	// Calculate Standard Deviation
	var varianceSum float64
	for _, val := range intervals {
		diff := val - meanSec
		varianceSum += diff * diff
	}
	stdDevSec := math.Sqrt(varianceSum / float64(len(intervals)))

	// Coefficient of Variation = sigma / mu
	cv := 0.0
	if meanSec > 0 {
		cv = stdDevSec / meanSec
	}
	cv = math.Round(cv*1000) / 1000.0

	// Determine Periodicity
	var periodicity PeriodicityClassification
	if cv <= 0.30 && len(intervals) >= 3 {
		periodicity = PeriodicityPeriodic
	} else if cv <= 0.80 {
		periodicity = PeriodicitySemiPeriodic
	} else {
		periodicity = PeriodicityIrregular
	}

	// Flapping check: more than 3 occurrences within 2 hours
	isFlapping := false
	if len(matching) >= 3 {
		totalSpan := timestamps[len(timestamps)-1].Sub(timestamps[0])
		if totalSpan <= 2*time.Hour {
			isFlapping = true
		}
	}

	conf := "high"
	if len(matching) < 3 {
		conf = "medium"
	}

	avgInterval := time.Duration(meanSec * float64(time.Second))
	medInterval := time.Duration(medianSec * float64(time.Second))
	stdDevInterval := time.Duration(stdDevSec * float64(time.Second))

	summary := fmt.Sprintf(
		"Recurring pattern observed across %d incidents (mean interval: %s, CV: %.2f, classification: %s, flapping: %t)",
		len(matching), avgInterval.Round(time.Minute), cv, periodicity, isFlapping,
	)

	return &RecurrenceAnalysis{
		PatternKey:             target.Title,
		OccurrenceCount:        len(matching),
		FirstOccurrence:        timestamps[0],
		MostRecentOccurrence:   timestamps[len(timestamps)-1],
		AverageInterval:        avgInterval,
		MedianInterval:         medInterval,
		StandardDeviation:      stdDevInterval,
		CoefficientOfVariation: cv,
		Periodicity:            periodicity,
		IsFlapping:             isFlapping,
		Confidence:             conf,
		HistoricalIncidentIDs:  incidentIDs,
		Summary:                summary,
	}
}

// BuildInvestigationReport compiles all analytical artifacts into a unified investigation report.
func BuildInvestigationReport(
	target Incident,
	history []Incident,
	nodeHostnames map[string]string,
	nodeTags map[string]map[string]string,
	totalFleetNodes int,
) IncidentInvestigationReport {
	now := time.Now().UTC()
	recurrence := AnalyzeRecurrence(target, history)
	similar := FindSimilarIncidents(target, history, 0.25, 5)
	impact := CalculateImpact(target.ID, target.AffectedNodes, nodeHostnames, nodeTags, target.RootSignals, totalFleetNodes)

	// Timeline highlights (first 5 and last 5 if large, otherwise full timeline)
	timeline := target.Timeline
	var highlights []IncidentTimelineEntry
	if len(timeline) <= 10 {
		highlights = timeline
	} else {
		highlights = append(highlights, timeline[:5]...)
		highlights = append(highlights, timeline[len(timeline)-5:]...)
	}

	findings := target.Findings
	if len(findings) == 0 {
		findings = GenerateIncidentFindings(&target)
	}

	summary := fmt.Sprintf(
		"Incident %s (%s, %s): %s. Affecting %d node(s) across %d subsystem(s). Recurrence: %s.",
		target.ID, target.Severity, target.Scope, target.Title, len(target.AffectedNodes), len(impact.Impact.Subsystems), recurrence.Periodicity,
	)

	return IncidentInvestigationReport{
		Incident:           target,
		Recurrence:         recurrence,
		SimilarIncidents:   similar,
		TimelineHighlights: highlights,
		Impact:             impact,
		Findings:           findings,
		Summary:            summary,
		GeneratedAt:        now,
	}
}
