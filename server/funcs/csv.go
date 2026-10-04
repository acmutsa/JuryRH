package funcs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"server/database"
	"server/models"
	"server/util"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/slices"
)

// Read CSV file and return a slice of judge structs
func ParseJudgeCSV(content string, hasHeader bool) ([]*models.Judge, error) {
	r := csv.NewReader(strings.NewReader(content))

	// Empty CSV file
	if content == "" {
		return []*models.Judge{}, nil
	}

	// If the CSV file has a header, skip the first line
	if hasHeader {
		r.Read()
	}

	// Read the CSV file, looping through each record
	var judges []*models.Judge
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) > 3 || strings.TrimSpace(record[0]) == "" {
			return nil, fmt.Errorf("judge CSV requires name, track (optional), notes (optional)")
		}
		track, notes := "", ""
		if len(record) >= 2 {
			track = strings.TrimSpace(record[1])
		}
		if len(record) >= 3 {
			notes = record[2]
		}
		judges = append(judges, models.NewJudge(strings.TrimSpace(record[0]), track, notes, -1))
	}

	return judges, nil
}

// Read CSV file and return a slice of project structs
func ParseProjectCsv(content string, hasHeader bool, db *mongo.Database) ([]*models.Project, error) {
	r := csv.NewReader(strings.NewReader(content))

	// Empty CSV file
	if content == "" {
		return []*models.Project{}, nil
	}

	// If the CSV file has a header, skip the first line
	if hasHeader {
		r.Read()
	}

	// Get the starting table number
	tableNum, err := database.GetMaxTableNum(db, context.Background())
	if err != nil {
		return nil, err
	}

	// Get options from the database
	options, err := database.GetOptions(db, context.Background())
	if err != nil {
		return nil, err
	}

	// Read the CSV file, looping through each record
	var projects []*models.Project
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Make sure the record has at least 3 elements (name, description, URL)
		if len(record) < 3 {
			return nil, fmt.Errorf("record contains less than 3 elements: '%s'", strings.Join(record, ","))
		}

		// Get the challenge list
		challengeList := []string{}
		if len(record) > 5 && record[5] != "" {
			challengeList = strings.Split(record[5], ",")
		}
		for i := range challengeList {
			challengeList[i] = strings.TrimSpace(challengeList[i])
		}

		// If the challenge list contains the ignore track, skip the project
		ignore := false
		for _, ignoreTrack := range options.IgnoreTracks {
			if slices.Contains(challengeList, ignoreTrack) {
				ignore = true
				break
			}
		}
		if ignore {
			continue
		}

		// Optional fields
		var tryLink string
		if len(record) > 3 && record[3] != "" {
			tryLink = record[3]
		}
		var videoLink string
		if len(record) > 4 && record[4] != "" {
			videoLink = record[4]
		}

		// Increment the table number
		tableNum++

		// Add project to slice
		projects = append(projects, models.NewProject(record[0], tableNum, util.GroupFromTable(options, tableNum), record[1], record[2], tryLink, videoLink, challengeList))
	}

	return projects, nil
}

// ParseDevpostCSV converts a Devpost Projects data export into Jury projects.
func ParseDevpostCSV(content string, db *mongo.Database) ([]*models.Project, error) {
	if content == "" {
		return []*models.Project{}, nil
	}

	// Get the starting table number
	tableNum, err := database.GetMaxTableNum(db, context.Background())
	if err != nil {
		return nil, err
	}

	// Get options from the database
	options, err := database.GetOptions(db, context.Background())
	if err != nil {
		return nil, err
	}

	return parseDevpostProjects(content, tableNum, options)
}

// Devpost's Projects data report calls challenge selections "Opt-In Prizes".
// Resolve the fields by header so added table numbers, PII, and custom questions do not
// shift the values imported into Jury.
func parseDevpostProjects(content string, tableNum int64, options *models.Options) ([]*models.Project, error) {
	r := csv.NewReader(strings.NewReader(content))
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading Devpost CSV header: %w", err)
	}

	columns := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if _, exists := columns[name]; exists {
			return nil, fmt.Errorf("duplicate Devpost CSV header %q", header[i])
		}
		columns[name] = i
	}

	required := []string{"project title", "submission url", "project status", "about the project", "opt-in prizes"}
	for _, name := range required {
		if _, exists := columns[name]; !exists {
			return nil, fmt.Errorf("missing required Devpost CSV header %q", name)
		}
	}

	value := func(record []string, name string) string {
		if i, exists := columns[name]; exists {
			return record[i]
		}
		return ""
	}

	// Read the CSV file, looping through each record
	var projects []*models.Project
	row := 1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading Devpost CSV row %d: %w", row+1, err)
		}
		row++

		// If the project is a Draft, skip it
		if strings.EqualFold(strings.TrimSpace(value(record, "project status")), "Draft") {
			continue
		}

		// Split challenge list into a slice and trim them
		prizes := value(record, "opt-in prizes")
		challengeList := strings.Split(prizes, ",")
		if strings.TrimSpace(prizes) == "" {
			challengeList = []string{}
		}
		for i := range challengeList {
			challengeList[i] = strings.TrimSpace(challengeList[i])
		}

		// If the challenge list contains the ignore track, skip the project
		ignore := false
		for _, ignoreTrack := range options.IgnoreTracks {
			if slices.Contains(challengeList, ignoreTrack) {
				ignore = true
				break
			}
		}
		if ignore {
			continue
		}

		// Increment the table number
		tableNum++

		// Add project to slice
		projects = append(projects, models.NewProject(
			value(record, "project title"),
			tableNum,
			util.GroupFromTable(options, tableNum),
			value(record, "about the project"),
			value(record, "submission url"),
			value(record, `"try it out" links`),
			value(record, "video demo link"),
			challengeList,
		))
	}

	return projects, nil
}

// AddCSVData adds a CSV file to the response
func AddCsvData(name string, content []byte, ctx *gin.Context) {
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", name))
	ctx.Header("Content-Type", "text/csv")
	ctx.Data(http.StatusOK, "text/csv", content)
}

// AddZipFile adds a zip file to the response
func AddZipFile(name string, content []byte, ctx *gin.Context) {
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", name))
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Data(http.StatusOK, "application/octet-stream", content)
}

// Create a CSV file from a list of judges
func CreateJudgeCSV(judges []*models.Judge) []byte {
	csvBuffer := &bytes.Buffer{}

	// Create a new CSV writer
	w := csv.NewWriter(csvBuffer)

	// Write the header
	// TODO: Add judge rankings to output
	w.Write([]string{"Name", "Notes", "Active", "ReadWelcome", "Seen", "LastActivity"})

	// Write each judge
	for _, judge := range judges {
		w.Write([]string{judge.Name, judge.Notes, fmt.Sprintf("%t", judge.Active), fmt.Sprintf("%t", judge.ReadWelcome), fmt.Sprintf("%d", judge.Seen), fmt.Sprintf("%d", judge.LastActivity)})
	}

	// Flush the writer
	w.Flush()

	return csvBuffer.Bytes()
}

// Create a CSV file from the judges but only the rankings
func CreateJudgeRankingCSV(judges []*models.Judge) []byte {
	csvBuffer := &bytes.Buffer{}

	// Create a new CSV writer
	w := csv.NewWriter(csvBuffer)

	// Write the header
	w.Write([]string{"Name", "Ranked", "Unranked"})

	// Write each judge
	for _, judge := range judges {
		// Don't include if their rankings are empty :/
		if len(judge.Rankings) == 0 {
			continue
		}

		// Create a list of all ranked projects (just their location)
		ranked := make([]int64, 0, len(judge.Rankings))
		for _, projId := range judge.Rankings {
			idx := util.IndexFunc(judge.SeenProjects, func(p models.JudgedProject) bool {
				return p.ProjectId == projId
			})
			if idx == -1 {
				continue
			}

			ranked = append(ranked, judge.SeenProjects[idx].Location)
		}

		// Create a list of all unranked projects (filter using ranked projects)
		unranked := make([]int64, 0, len(judge.SeenProjects)-len(judge.Rankings))
		for _, proj := range judge.SeenProjects {
			if util.ContainsFunc(ranked, func(table int64) bool { return table == proj.Location }) {
				unranked = append(unranked, proj.Location)
			}
		}

		// Convert arrays to strings
		rankedStr := util.IntToString(ranked)
		unrankedStr := util.IntToString(unranked)

		// Write line to CSV
		w.Write([]string{judge.Name, strings.Join(rankedStr, ","), strings.Join(unrankedStr, ",")})
	}

	// Flush the writer
	w.Flush()

	return csvBuffer.Bytes()
}

// ProjectExportOptions selects the judging pools included in project exports.
type ProjectExportOptions struct {
	Tracks      []string
	Challenges  []string
	Nominations map[string][]database.ChallengeNomination
}

// Create a CSV file from projects with their current judging results.
func CreateProjectCSV(projects []*models.Project, options ProjectExportOptions) []byte {
	csvBuffer := &bytes.Buffer{}

	// Create a new CSV writer
	w := csv.NewWriter(csvBuffer)

	// Write the header
	header := []string{"Name", "Table", "Description", "URL", "TryLink", "VideoLink", "ChallengeList", "Seen", "Active", "LastActivity", "Score", "Stars"}
	for _, track := range options.Tracks {
		header = append(header, "Track Score: "+track, "Track Stars: "+track, "Track Seen: "+track)
	}
	challengeStars := make(map[string]map[string]int)
	for _, challenge := range options.Challenges {
		header = append(header, "Challenge Stars: "+challenge)
		challengeStars[challenge] = make(map[string]int)
		for _, nomination := range options.Nominations[challenge] {
			challengeStars[challenge][nomination.ProjectID] = nomination.Stars
		}
	}
	w.Write(header)

	// Write each project
	for _, project := range projects {
		row := []string{project.Name, fmt.Sprintf("Table %d", project.Location), project.Description, project.Url, project.TryLink, project.VideoLink, strings.Join(project.ChallengeList, ","), fmt.Sprintf("%d", project.Seen), fmt.Sprintf("%t", project.Active), fmt.Sprintf("%d", project.LastActivity), fmt.Sprintf("%d", project.Score), fmt.Sprintf("%d", project.Stars)}
		for _, track := range options.Tracks {
			if contains(project.ChallengeList, track) {
				row = append(row, fmt.Sprintf("%d", project.TrackScores[track]), fmt.Sprintf("%d", project.TrackStars[track]), fmt.Sprintf("%d", project.TrackSeen[track]))
			} else {
				row = append(row, "", "", "")
			}
		}
		for _, challenge := range options.Challenges {
			if contains(project.ChallengeList, challenge) {
				row = append(row, fmt.Sprintf("%d", challengeStars[challenge][project.Id.Hex()]))
			} else {
				row = append(row, "")
			}
		}
		w.Write(row)
	}

	// Flush the writer
	w.Flush()

	return csvBuffer.Bytes()
}

// CreateProjectChallengeZip creates a zip file with a CSV for each challenge
func CreateProjectChallengeZip(projects []*models.Project, options ProjectExportOptions) ([]byte, error) {
	csvList := [][]byte{}

	// Get list of challenges
	challengeList := []string{}
	for _, project := range projects {
		for _, challenge := range project.ChallengeList {
			if !contains(challengeList, challenge) {
				challengeList = append(challengeList, challenge)
			}
		}
	}

	// Create a CSV for each challenge
	for _, challenge := range challengeList {
		currChallengeProjects := []*models.Project{}
		for _, project := range projects {
			if contains(project.ChallengeList, challenge) {
				currChallengeProjects = append(currChallengeProjects, project)
			}
		}

		// Create CSV for the challenge
		csv := CreateProjectCSV(currChallengeProjects, options)
		csvList = append(csvList, csv)
	}

	// Create buffer for zip file
	zipBuffer := &bytes.Buffer{}

	// Create a new zip writer
	w := zip.NewWriter(zipBuffer)

	// Write each CSV to the zip file
	for i, csv := range csvList {
		f, err := w.Create(fmt.Sprintf("%s.csv", challengeList[i]))
		if err != nil {
			return nil, err
		}

		_, err = f.Write(csv)
		if err != nil {
			return nil, err
		}
	}

	// Close the zip writer
	err := w.Close()
	if err != nil {
		return nil, err
	}

	return zipBuffer.Bytes(), nil
}

// contains checks if a string is in a list of strings
func contains(list []string, str string) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}
	return false
}
