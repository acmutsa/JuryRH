package funcs

import (
	"encoding/csv"
	"reflect"
	"server/models"
	"strings"
	"testing"
)

func devpostCSV(t *testing.T, rows ...[]string) string {
	t.Helper()
	var output strings.Builder
	w := csv.NewWriter(&output)
	if err := w.WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestParseDevpostProjectsUsesHeaders(t *testing.T) {
	options := models.NewOptions()
	options.IgnoreTracks = []string{"Ignore Me"}
	content := devpostCSV(t,
		[]string{"\ufeffOpt-In Prizes", "Custom Question", "Project Status", "Video Demo Link", "Project Title", "Submission Url", `"Try it out" Links`, "About The Project", "Assigned Table Number"},
		[]string{"Track A, Track B", "answer", "Submitted", "https://video.example", "First", "https://devpost.com/first", "https://demo.example", "First description", "99"},
		[]string{"", "", "Draft", "", "Draft Project", "https://devpost.com/draft", "", "Draft description", "100"},
		[]string{"Ignore Me, Track A", "", "Submitted", "", "Ignored Project", "https://devpost.com/ignored", "", "Ignored description", "101"},
		[]string{"", "", "Submitted", "", "Second", "https://devpost.com/second", "", "Second description", "102"},
	)

	projects, err := parseDevpostProjects(content, 7, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	first := projects[0]
	if first.Name != "First" || first.Location != 8 || first.Description != "First description" || first.Url != "https://devpost.com/first" || first.TryLink != "https://demo.example" || first.VideoLink != "https://video.example" {
		t.Errorf("wrong first project: %+v", first)
	}
	if !reflect.DeepEqual(first.ChallengeList, []string{"Track A", "Track B"}) {
		t.Errorf("wrong challenges: %v", first.ChallengeList)
	}
	if projects[1].Name != "Second" || projects[1].Location != 9 || len(projects[1].ChallengeList) != 0 {
		t.Errorf("wrong second project: %+v", projects[1])
	}
}

func TestParseDevpostProjectsMissingHeader(t *testing.T) {
	content := devpostCSV(t,
		[]string{"Project Title", "Submission Url", "Project Status", "About The Project"},
		[]string{"First", "https://devpost.com/first", "Submitted", "Description"},
	)
	_, err := parseDevpostProjects(content, 0, models.NewOptions())
	if err == nil || !strings.Contains(err.Error(), "opt-in prizes") {
		t.Fatalf("expected a missing Opt-In Prizes error, got %v", err)
	}
}

func TestParseDevpostProjectsDuplicateHeader(t *testing.T) {
	content := devpostCSV(t,
		[]string{"Project Title", "project title", "Submission Url", "Project Status", "About The Project", "Opt-In Prizes"},
	)
	_, err := parseDevpostProjects(content, 0, models.NewOptions())
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected a duplicate header error, got %v", err)
	}
}

func TestParseDevpostProjectsOptionalLinks(t *testing.T) {
	content := devpostCSV(t,
		[]string{"Project Title", "Submission Url", "Project Status", "About The Project", "Opt-In Prizes"},
		[]string{"First", "https://devpost.com/first", "Submitted", "Description", "Track A"},
	)
	projects, err := parseDevpostProjects(content, 0, models.NewOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].TryLink != "" || projects[0].VideoLink != "" {
		t.Fatalf("expected one project with empty optional links, got %+v", projects)
	}
}
