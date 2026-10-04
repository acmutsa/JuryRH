package funcs

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"reflect"
	"server/database"
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

func TestParseJudgeCSVWithoutEmail(t *testing.T) {
	judges, err := ParseJudgeCSV("Name,Track,Notes\nAlex,Design,Panel lead\nSam,,\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(judges) != 2 || judges[0].Name != "Alex" || judges[0].Track != "Design" || judges[0].Notes != "Panel lead" || judges[1].Track != "" {
		t.Fatalf("unexpected judges: %+v", judges)
	}
	if _, err := ParseJudgeCSV("   ,Design,\n", false); err == nil {
		t.Fatal("blank judge name accepted")
	}
	csv := CreateJudgeCSV(judges)
	if strings.Contains(string(csv), "Email") {
		t.Fatal("export still contains email column")
	}
}

func TestProjectExportsIncludeJudgingResults(t *testing.T) {
	project := models.NewProject("Rated, project", 1, 0, "", "", "", "", []string{"Design", "Climate"})
	project.Id = primitive.NewObjectID()
	project.Score, project.Stars = -2, 3
	project.TrackScores["Design"], project.TrackStars["Design"], project.TrackSeen["Design"] = 4, 1, 2
	unseen := models.NewProject("Unseen", 2, 0, "", "", "", "", []string{"Climate"})
	unseen.Id = primitive.NewObjectID()
	options := ProjectExportOptions{
		Tracks: []string{"Design", "Other"}, Challenges: []string{"Climate"},
		Nominations: map[string][]database.ChallengeNomination{
			"Climate":  {{ProjectID: project.Id.Hex(), Stars: 2}},
			"Disabled": {{ProjectID: project.Id.Hex(), Stars: 99}},
		},
	}
	projects := []*models.Project{project, unseen}
	read := func(data []byte) [][]string {
		records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return records
	}
	records := read(CreateProjectCSV(projects, options))
	header := records[0]
	if strings.Join(header[:4], ",") != "Name,Table,Score,Stars" {
		t.Fatal("judging results should be visible beside project names")
	}
	for _, column := range header {
		if column == "Challenge Stars: Disabled" {
			t.Fatal("disabled challenge exported")
		}
	}
	for i, record := range records[1:] {
		values := make(map[string]string)
		for j, column := range header {
			values[column] = record[j]
		}
		expected := map[string]string{"Score": "-2", "Stars": "3", "Track Score: Design": "4", "Track Stars: Design": "1", "Track Seen: Design": "2", "Challenge Stars: Climate": "2", "Track Score: Other": ""}
		if i == 1 {
			expected = map[string]string{"Score": "0", "Stars": "0", "Track Score: Design": "", "Challenge Stars: Climate": "0"}
		}
		for column, want := range expected {
			if values[column] != want {
				t.Errorf("row %d %s = %q, want %q", i, column, values[column], want)
			}
		}
	}
	data, err := CreateProjectChallengeZip(projects, options)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("expected 2 challenge files, got %d", len(archive.File))
	}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		rows := read(content)
		if !reflect.DeepEqual(rows[0], header) {
			t.Fatalf("%s has different judging columns", file.Name)
		}
		if !reflect.DeepEqual(rows[1], records[1]) {
			t.Fatalf("%s lost judging totals", file.Name)
		}
		if file.Name == "Design.csv" && len(rows) != 2 {
			t.Fatal("Design export includes a project outside the track")
		}
		if file.Name == "Climate.csv" && len(rows) != 3 {
			t.Fatal("Climate export is missing a project")
		}
	}
}
