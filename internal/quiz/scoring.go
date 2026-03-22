package quiz

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/state"
)

// ComponentScore holds a scored component candidate together with its section,
// recent question history, and derived statistics used by the orchestrator.
type ComponentScore struct {
	Component        state.Component
	Section          state.Section
	RecentHistory    []state.QuestionHistoryEntry // up to recentHistoryN most recent
	Attempts         int
	Accuracy         float64 // 0.0–1.0; 0.5 when never attempted (neutral)
	DaysSinceAttempt float64 // 999 when never attempted
	Score            float64
}

const recentHistoryN = 5

// ScoreComponents scores every component for class using a composite formula
// that balances weakness, novelty of exposure, and recency of last attempt:
//
//	adjustedAccuracy = accuracy if attempted, else 0.5 (neutral)
//	noveltyFactor    = 1 / (log2(attempts+1) + 1)
//	recencyFactor    = min(log2(daysSince+1) / log2(90), 1.0)
//	score            = 0.40*(1-adjustedAccuracy) + 0.35*noveltyFactor + 0.25*recencyFactor
//
// The returned slice is sorted descending by score.
func ScoreComponents(class string, secIdx *state.SectionIndex, cmpIdx *state.ComponentIndex) []ComponentScore {
	sectionByID := make(map[string]state.Section, len(secIdx.Sections))
	for _, s := range secIdx.Sections {
		if strings.EqualFold(s.Class, class) {
			sectionByID[s.ID] = s
		}
	}

	now := time.Now().UTC()
	var scores []ComponentScore

	for _, cmp := range cmpIdx.Components {
		if !strings.EqualFold(cmp.Class, class) {
			continue
		}

		history := cmp.QuestionHistory
		attempts := len(history)
		recent := recentHistoryEntries(history, recentHistoryN)

		var adjustedAccuracy float64
		if attempts == 0 {
			adjustedAccuracy = 0.5 // neutral — never tried
		} else {
			correct := 0
			for _, h := range history {
				if h.Correct {
					correct++
				}
			}
			adjustedAccuracy = float64(correct) / float64(attempts)
		}

		// Novelty: diminishes as repeated exposure grows.
		noveltyFactor := 1.0 / (math.Log2(float64(attempts)+1) + 1)

		// Recency: grows the longer since last attempt, using a logarithmic
		// curve over 90 days so material from 2 weeks ago and 3 months ago
		// are not treated equally once they pass a short threshold.
		daysSince := 999.0 // never attempted → maximum recency signal
		if attempts > 0 {
			lastAttempt := history[len(history)-1].AnsweredAt
			daysSince = now.Sub(lastAttempt).Hours() / 24
		}
		recencyFactor := math.Min(math.Log2(daysSince+1)/math.Log2(90), 1.0)

		score := 0.40*(1-adjustedAccuracy) + 0.35*noveltyFactor + 0.25*recencyFactor

		scores = append(scores, ComponentScore{
			Component:        cmp,
			Section:          sectionByID[cmp.SectionID],
			RecentHistory:    recent,
			Attempts:         attempts,
			Accuracy:         adjustedAccuracy,
			DaysSinceAttempt: daysSince,
			Score:            score,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	return scores
}

// SelectCandidates returns the top maxN scored components.
// If maxN <= 0 or len(scores) <= maxN, the full slice is returned.
func SelectCandidates(scores []ComponentScore, maxN int) []ComponentScore {
	if maxN <= 0 || len(scores) <= maxN {
		return scores
	}
	return scores[:maxN]
}

func recentHistoryEntries(history []state.QuestionHistoryEntry, n int) []state.QuestionHistoryEntry {
	if len(history) == 0 {
		return nil
	}
	if len(history) <= n {
		return history
	}
	return history[len(history)-n:]
}
