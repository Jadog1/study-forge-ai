package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/state"
)

type knowledgeQuizMetrics struct {
	Attempts     int
	Correct      int
	Incorrect    int
	Accuracy     float64
	LastAnswered time.Time
}

func knowledgeMetricsFromHistory(history []state.QuestionHistoryEntry) knowledgeQuizMetrics {
	metrics := knowledgeQuizMetrics{Attempts: len(history)}
	for _, item := range history {
		if item.Correct {
			metrics.Correct++
		} else {
			metrics.Incorrect++
		}
		if item.AnsweredAt.After(metrics.LastAnswered) {
			metrics.LastAnswered = item.AnsweredAt
		}
	}
	if metrics.Attempts > 0 {
		metrics.Accuracy = float64(metrics.Correct) / float64(metrics.Attempts)
	}
	return metrics
}

func emptyFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func knowledgePaneBorderStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)
	if active {
		style = style.BorderForeground(colorBorderFocus)
	}
	return style
}

func knowledgeSectionRowStyle(selected, active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(colorText)
	if selected {
		style = style.Background(colorSurfaceLt).Foreground(colorSecondary)
	}
	if selected && active {
		style = style.Bold(true)
	}
	return style
}
