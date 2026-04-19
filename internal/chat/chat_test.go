package chat

import "testing"

func TestNormalizeMode_DefaultAndAliases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Mode
	}{
		{name: "default empty", in: "", want: ModeStandard},
		{name: "socratic", in: "socratic", want: ModeSocratic},
		{name: "explain underscore", in: "explain_back", want: ModeExplainBack},
		{name: "explain hyphen", in: "explain-back", want: ModeExplainBack},
		{name: "explain spaced", in: "explain back", want: ModeExplainBack},
		{name: "unknown falls back", in: "something-else", want: ModeStandard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeMode(tt.in); got != tt.want {
				t.Fatalf("NormalizeMode(%q)=%q want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToolQuizDirectives_UsesTopLevelCountForSingleDirective(t *testing.T) {
	directives, err := toolQuizDirectives(map[string]any{
		"count": 1,
		"directives": []any{
			map[string]any{"component_id": "cmp-1"},
		},
	}, 1, "multiple-choice")
	if err != nil {
		t.Fatalf("toolQuizDirectives returned error: %v", err)
	}
	if len(directives) != 1 {
		t.Fatalf("expected 1 directive, got %d", len(directives))
	}
	if directives[0].QuestionCount != 1 {
		t.Fatalf("expected question_count 1, got %d", directives[0].QuestionCount)
	}
	if len(directives[0].QuestionTypes) != 1 || directives[0].QuestionTypes[0] != "multiple-choice" {
		t.Fatalf("expected inherited question type, got %#v", directives[0].QuestionTypes)
	}
}

func TestToolQuizDirectives_ParsesManualPlan(t *testing.T) {
	directives, err := toolQuizDirectives(map[string]any{
		"directives": []any{
			map[string]any{
				"component_id":   "cmp-1",
				"section_id":     "sec-1",
				"section_title":  "Intro",
				"question_count": 2,
				"question_types": []any{"short-answer", "true-false"},
				"angle":          "demo",
			},
		},
	}, 0, "multiple-choice")
	if err != nil {
		t.Fatalf("toolQuizDirectives returned error: %v", err)
	}
	if len(directives) != 1 {
		t.Fatalf("expected 1 directive, got %d", len(directives))
	}
	if directives[0].ComponentID != "cmp-1" || directives[0].SectionID != "sec-1" {
		t.Fatalf("expected component and section ids to parse, got %#v", directives[0])
	}
	if directives[0].QuestionCount != 2 {
		t.Fatalf("expected question_count 2, got %d", directives[0].QuestionCount)
	}
	if len(directives[0].QuestionTypes) != 2 {
		t.Fatalf("expected question types to parse, got %#v", directives[0].QuestionTypes)
	}
	if directives[0].Angle != "demo" {
		t.Fatalf("expected angle to parse, got %q", directives[0].Angle)
	}
}

func TestToolQuizDirectives_RejectsInvalidShape(t *testing.T) {
	_, err := toolQuizDirectives(map[string]any{
		"directives": map[string]any{"component_id": "cmp-1"},
	}, 0, "multiple-choice")
	if err == nil {
		t.Fatal("expected invalid directives error, got nil")
	}
}
