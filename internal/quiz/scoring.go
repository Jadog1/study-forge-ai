package quiz

import (
	"math"
	"math/rand"
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
	RecentAccuracy   float64 // 0.0–1.0 over RecentHistory window; 0.5 when empty
	IncorrectStreak  int     // consecutive incorrect answers from most recent backwards
	ThoughtProvoking float64 // 0.0–1.0 share of recent questions with higher-order cues
	DifficultyBand   string  // supportive, balanced, advanced
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
		recentAccuracy := accuracyFromHistory(recent)
		incorrectStreak := recentIncorrectStreak(recent)
		thoughtProvoking := thoughtProvokingRate(recent)
		difficultyBand := deriveDifficultyBand(attempts, recentAccuracy, incorrectStreak)

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
			RecentAccuracy:   recentAccuracy,
			IncorrectStreak:  incorrectStreak,
			ThoughtProvoking: thoughtProvoking,
			DifficultyBand:   difficultyBand,
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

// SelectCandidatesDiversified returns a top-weighted candidate subset while
// reserving a fraction of slots for exploration from a wider high-score window.
//
// explorationRate is clamped to [0.0, 1.0]. When zero, this behaves like
// SelectCandidates. The rng parameter is optional and only used for tests.
func SelectCandidatesDiversified(scores []ComponentScore, maxN int, explorationRate float64, rng *rand.Rand) []ComponentScore {
	if maxN <= 0 || len(scores) <= maxN {
		return scores
	}
	if explorationRate <= 0 {
		return SelectCandidates(scores, maxN)
	}
	if explorationRate > 1 {
		explorationRate = 1
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	anchorCount := int(math.Round(float64(maxN) * (1 - explorationRate)))
	if anchorCount < 1 {
		anchorCount = 1
	}
	if anchorCount > maxN {
		anchorCount = maxN
	}

	out := make([]ComponentScore, 0, maxN)
	out = append(out, scores[:anchorCount]...)

	remaining := maxN - anchorCount
	if remaining <= 0 {
		return out
	}

	windowEnd := maxN * 6
	if windowEnd > len(scores) {
		windowEnd = len(scores)
	}
	if windowEnd <= anchorCount {
		windowEnd = len(scores)
	}
	pool := append([]ComponentScore(nil), scores[anchorCount:windowEnd]...)
	if len(pool) == 0 {
		return out
	}

	// Shuffle exploration pool so each quiz run sees a slightly different mix.
	rng.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})

	if remaining > len(pool) {
		remaining = len(pool)
	}
	out = append(out, pool[:remaining]...)
	return out
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

func accuracyFromHistory(history []state.QuestionHistoryEntry) float64 {
	if len(history) == 0 {
		return 0.5
	}
	correct := 0
	for _, h := range history {
		if h.Correct {
			correct++
		}
	}
	return float64(correct) / float64(len(history))
}

func recentIncorrectStreak(history []state.QuestionHistoryEntry) int {
	streak := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Correct {
			break
		}
		streak++
	}
	return streak
}

func thoughtProvokingRate(history []state.QuestionHistoryEntry) float64 {
	if len(history) == 0 {
		return 0
	}
	keywords := []string{
		"why", "how", "compare", "contrast", "explain", "justify", "predict", "evaluate", "trade-off", "scenario",
	}
	hits := 0
	for _, h := range history {
		question := strings.ToLower(strings.TrimSpace(h.Question))
		if question == "" {
			continue
		}
		for _, keyword := range keywords {
			if strings.Contains(question, keyword) {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(history))
}

func deriveDifficultyBand(attempts int, recentAccuracy float64, incorrectStreak int) string {
	if attempts >= 2 && (recentAccuracy <= 0.45 || incorrectStreak >= 2) {
		return "supportive"
	}
	if attempts >= 3 && recentAccuracy >= 0.80 && incorrectStreak == 0 {
		return "advanced"
	}
	return "balanced"
}
