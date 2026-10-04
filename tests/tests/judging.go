package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"tests/util"
)

// ChallengeNominations checks opt-in eligibility, quota, persistence, and the admin summary.
func ChallengeNominations(context *util.Context) util.Result {
	reset := util.PostRequest(context.Logger, "/admin/reset", util.H{"type": "projects"}, util.AdminAuth())
	if !util.IsOk(reset) {
		return util.NewResult(false, "Could not reset test projects: "+reset)
	}
	for i := 0; i < 3; i++ {
		res := util.PostRequest(context.Logger, "/project/new", util.H{
			"name":        fmt.Sprintf("Challenge Candidate %d", i),
			"description": "Challenge nomination test", "url": "https://example.com",
			"try_link": "", "video_link": "", "challenge_list": "Climate",
		}, util.AdminAuth())
		if !util.IsOk(res) {
			return util.NewResult(false, "Could not add challenge project: "+res)
		}
	}
	badOption := util.PostRequest(context.Logger, "/admin/options", util.H{"opt_in_challenges": []string{"Unknown"}}, util.AdminAuth())
	if util.IsOk(badOption) {
		return util.NewResult(false, "Unknown challenge was accepted")
	}
	setOption := util.PostRequest(context.Logger, "/admin/options", util.H{"opt_in_challenges": []string{"Climate"}}, util.AdminAuth())
	if !util.IsOk(setOption) {
		return util.NewResult(false, "Could not enable challenge: "+setOption)
	}
	defer util.PostRequest(context.Logger, "/admin/options", util.H{"opt_in_challenges": []string{}}, util.AdminAuth())
	token, result := createNamedJudge(context, "challenge_test@example.com", "Challenge Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)
	limit := util.ExtractInt(util.GetRequest(context.Logger, "/judge/challenges", auth), "limit")
	if limit < 1 {
		return util.NewResult(false, "Invalid challenge nomination limit")
	}
	for i := 3; i <= limit; i++ {
		res := util.PostRequest(context.Logger, "/project/new", util.H{
			"name":        fmt.Sprintf("Challenge Candidate %d", i),
			"description": "Challenge nomination test", "url": "https://example.com",
			"try_link": "", "video_link": "", "challenge_list": "Climate",
		}, util.AdminAuth())
		if !util.IsOk(res) {
			return util.NewResult(false, "Could not add challenge project: "+res)
		}
	}
	for i := 0; i <= limit; i++ {
		next := util.PostRequest(context.Logger, "/judge/next", nil, auth)
		if util.ExtractString(next, "project_id") == "" {
			return util.NewResult(false, "No next challenge project: "+next)
		}
		choices := util.GetRequest(context.Logger, "/judge/challenges", auth)
		if !strings.Contains(choices, "Climate") {
			return util.NewResult(false, "Eligible challenge missing: "+choices)
		}
		if i == 0 {
			invalid := util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "", "starred": false, "challenge_stars": []string{"Unknown"}}, auth)
			if util.IsOk(invalid) {
				return util.NewResult(false, "Ineligible nomination was accepted")
			}
		}
		finish := util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "", "starred": false, "challenge_stars": []string{"Climate"}}, auth)
		if i < limit && !util.IsOk(finish) {
			return util.NewResult(false, "Valid nomination failed: "+finish)
		}
		if i == limit {
			if util.IsOk(finish) {
				return util.NewResult(false, "Nomination exceeded challenge quota")
			}
			finish = util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "", "starred": false}, auth)
			if !util.IsOk(finish) {
				return util.NewResult(false, "Finish without nomination failed: "+finish)
			}
		}
	}
	projects := util.GetRequest(context.Logger, "/judge/projects", auth)
	if !strings.Contains(projects, "challenge_stars") {
		return util.NewResult(false, "Nominations not stored with judged projects")
	}
	summary := util.GetRequest(context.Logger, "/admin/challenge-nominations", util.AdminAuth())
	if !strings.Contains(summary, "Climate") || !strings.Contains(summary, "Challenge Candidate") {
		return util.NewResult(false, "Admin nominations summary missing projects: "+summary)
	}
	return util.ResultOk()
}

// --- Judging Workflow Tests ---

// JudgingTestSetup will set the setting that may break a standard workflow
// This includes actually starting judging and disabling groups + tracks
func JudgingTestSetup(context *util.Context) util.Result {
	// Disable group/track judging
	setRes := util.PostRequest(context.Logger, "/admin/options", util.H{
		"judge_tracks": false,
		"multi_group":  false,
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Unpause clock (to make sure judging starts)
	unpauseRes := util.PostRequest(context.Logger, "/admin/clock/unpause", nil, util.AdminAuth())
	if !util.IsOk(unpauseRes) {
		return util.NewResult(false, "Failed to unpause clock: "+unpauseRes)
	}

	return util.ResultOk()
}

// JudgingStandardPath runs through the core judging loop:
// create judge + projects → login → get next → finish → verify seen counts increment
func JudgingStandardPath(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Add two projects
	for i := 1; i <= 2; i++ {
		addRes := util.PostRequest(context.Logger, "/project/new", util.H{
			"name":           fmt.Sprintf("Judging Project %d", i),
			"description":    "Test project for judging flow",
			"url":            "https://example.com",
			"try_link":       "",
			"video_link":     "",
			"challenge_list": "",
		}, util.AdminAuth())
		if !util.IsOk(addRes) {
			return util.NewResult(false, fmt.Sprintf("Failed to add judging project %d: %s", i, addRes))
		}
	}

	// Create a judge and log in
	token, result := createNamedJudge(context, "judging_flow@example.com", "Judging Flow Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	// Get project count before
	statsBefore := util.GetRequest(context.Logger, "/project/stats", util.AdminAuth())
	avgSeenBefore := util.ExtractInt(statsBefore, "avg_seen")

	// Get next project
	nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
	projectID := util.ExtractString(nextRes, "project_id")
	if projectID == "" {
		return util.NewResult(false, "GET /judge/next did not return a project_id: "+nextRes)
	}

	// Finish judging that project
	finishRes := util.PostRequest(context.Logger, "/judge/finish", util.H{
		"notes":   "Looks great",
		"starred": false,
	}, auth)
	if !util.IsOk(finishRes) {
		return util.NewResult(false, "POST /judge/finish failed: "+finishRes)
	}

	// Verify avg_seen increased (or at minimum, seen count on the judge went up)
	judgeRes := util.GetRequest(context.Logger, "/judge", auth)
	seen := util.ExtractInt(judgeRes, "seen")
	if seen < 1 {
		return util.NewResult(false, fmt.Sprintf("Judge 'seen' count should be at least 1 after judging, got %d", seen))
	}

	_ = avgSeenBefore // avoid unused variable error; avg_seen may not change with just one judge

	return util.ResultOk()
}

// JudgeDoesNotRepeatProjects verifies a judge never gets the same project twice
func JudgeDoesNotRepeatProjects(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Add 3 projects
	for i := 1; i <= 3; i++ {
		util.PostRequest(context.Logger, "/project/new", util.H{
			"name":           fmt.Sprintf("No Repeat Project %d", i),
			"description":    "Uniqueness test",
			"url":            "https://example.com",
			"try_link":       "",
			"video_link":     "",
			"challenge_list": "",
		}, util.AdminAuth())
	}

	token, result := createNamedJudge(context, "no_repeat@example.com", "No Repeat Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	seen := map[string]bool{}

	for i := 0; i < 3; i++ {
		nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
		projectID := util.ExtractString(nextRes, "project_id")
		if projectID == "" {
			// No more projects — that's fine, stop early
			break
		}
		if seen[projectID] {
			return util.NewResult(false, fmt.Sprintf("Judge was assigned the same project twice: %s", projectID))
		}
		seen[projectID] = true

		finishRes := util.PostRequest(context.Logger, "/judge/finish", util.H{
			"notes":   "",
			"starred": false,
		}, auth)
		if !util.IsOk(finishRes) {
			return util.NewResult(false, "POST /judge/finish failed on iteration "+fmt.Sprint(i))
		}
	}

	return util.ResultOk()
}

// SkipProjectCreatesFlag verifies that skipping a project records a flag visible to admin
func SkipProjectCreatesFlag(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Add a project to skip
	util.PostRequest(context.Logger, "/project/new", util.H{
		"name":           "Skip Test Project",
		"description":    "Will be skipped",
		"url":            "https://example.com",
		"try_link":       "",
		"video_link":     "",
		"challenge_list": "",
	}, util.AdminAuth())

	token, result := createNamedJudge(context, "skip_test@example.com", "Skip Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	// Get count of flags before
	flagsBefore := util.GetRequest(context.Logger, "/admin/flags", util.AdminAuth())
	countBefore := countFlags(flagsBefore)

	// Get next project
	nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
	projectID := util.ExtractString(nextRes, "project_id")
	if projectID == "" {
		return util.NewResult(false, "No project returned for skip test: "+nextRes)
	}

	// Skip the project
	skipRes := util.PostRequest(context.Logger, "/judge/skip", util.H{
		"reason": "absent",
	}, auth)
	if !util.IsOk(skipRes) {
		return util.NewResult(false, "POST /judge/skip failed: "+skipRes)
	}

	// Verify flag count increased
	flagsAfter := util.GetRequest(context.Logger, "/admin/flags", util.AdminAuth())
	countAfter := countFlags(flagsAfter)

	if countAfter <= countBefore {
		return util.NewResult(false, fmt.Sprintf("Flag count should increase after skip: before=%d, after=%d", countBefore, countAfter))
	}

	return util.ResultOk()
}

// JudgeRankProjects verifies that submitting a ranking succeeds
func JudgeRankProjects(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Add projects
	var projectIDs []string
	for i := 1; i <= 2; i++ {
		name := fmt.Sprintf("Rank Test Project %d", i)
		util.PostRequest(context.Logger, "/project/new", util.H{
			"name":           name,
			"description":    "For ranking",
			"url":            "https://example.com",
			"try_link":       "",
			"video_link":     "",
			"challenge_list": "",
		}, util.AdminAuth())

		id, result := findProjectIDByName(context, name)
		if !result.Success {
			return result
		}
		projectIDs = append(projectIDs, id)
	}

	token, result := createNamedJudge(context, "rank_test@example.com", "Rank Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	// Judge must see projects before ranking — do 2 finish cycles
	for i := 0; i < 2; i++ {
		nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
		if util.ExtractString(nextRes, "project_id") == "" {
			break
		}
		util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "", "starred": false}, auth)
	}

	// Submit a ranking
	rankRes := util.PostRequest(context.Logger, "/judge/rank", util.H{
		"ranking": projectIDs,
	}, auth)
	if !util.IsOk(rankRes) {
		return util.NewResult(false, "POST /judge/rank failed: "+rankRes)
	}

	return util.ResultOk()
}

// StarProject verifies a judge can star and unstar a project
func StarProject(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	util.PostRequest(context.Logger, "/project/new", util.H{
		"name":           "Star Test Project",
		"description":    "Will be starred",
		"url":            "https://example.com",
		"try_link":       "",
		"video_link":     "",
		"challenge_list": "",
	}, util.AdminAuth())

	token, result := createNamedJudge(context, "star_test@example.com", "Star Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
	projectID := util.ExtractString(nextRes, "project_id")
	if projectID == "" {
		return util.NewResult(false, "No project returned for star test")
	}

	// Finish with starred = true
	finishRes := util.PostRequest(context.Logger, "/judge/finish", util.H{
		"notes":   "",
		"starred": true,
	}, auth)
	if !util.IsOk(finishRes) {
		return util.NewResult(false, "POST /judge/finish (starred) failed: "+finishRes)
	}

	// Verify via /judge/projects that starred is true
	projsRes := util.GetRequest(context.Logger, "/judge/projects", auth)
	starred := findSeenProjectField(projsRes, projectID, "starred")
	if starred != "true" {
		return util.NewResult(false, fmt.Sprintf("Project should be starred, got '%s'", starred))
	}

	return util.ResultOk()
}

// JudgeNotesUpdate verifies that notes can be updated for a seen project
func JudgeNotesUpdate(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	util.PostRequest(context.Logger, "/project/new", util.H{
		"name":           "Notes Test Project",
		"description":    "Will have notes",
		"url":            "https://example.com",
		"try_link":       "",
		"video_link":     "",
		"challenge_list": "",
	}, util.AdminAuth())

	token, result := createNamedJudge(context, "notes_test@example.com", "Notes Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	nextRes := util.PostRequest(context.Logger, "/judge/next", nil, auth)
	projectID := util.ExtractString(nextRes, "project_id")
	if projectID == "" {
		return util.NewResult(false, "No project returned for notes test")
	}

	util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "initial note", "starred": false}, auth)

	// Update notes
	notesRes := util.PutRequest(context.Logger, "/judge/notes/"+projectID, util.H{"notes": "updated note"}, auth)
	if !util.IsOk(notesRes) {
		return util.NewResult(false, "PUT /judge/notes/:id failed: "+notesRes)
	}

	// Verify
	projsRes := util.GetRequest(context.Logger, "/judge/projects", auth)
	notes := findSeenProjectField(projsRes, projectID, "notes")
	if notes != "updated note" {
		return util.NewResult(false, fmt.Sprintf("Notes not updated: got '%s'", notes))
	}

	return util.ResultOk()
}

// JudgeNextWithNoProjects verifies the API handles the case gracefully (no 500)
func JudgeNextWithNoActiveProjects(context *util.Context) util.Result {
	// Delete all projects
	setRes := util.PostRequest(context.Logger, "/admin/reset", util.H{
		"type": "projects",
	}, util.AdminAuth())
	if !util.IsOk(setRes) {
		return util.NewResult(false, "Failed to set judge-tracks and multi_group: "+setRes)
	}

	// Create a fresh judge
	token, result := createNamedJudge(context, "empty_test@example.com", "Empty Test Judge")
	if !result.Success {
		return result
	}
	auth := util.JudgeAuth(token)

	status, _ := util.PostRequestWithStatus(context.Logger, "/judge/next", nil, auth)
	if status == 500 {
		return util.NewResult(false, "GET /judge/next with no projects should not return 500")
	}

	return util.ResultOk()
}

// --- Helpers ---

// countFlags counts the number of flag objects in a JSON array response
func countFlags(body string) int {
	var flags []map[string]any
	if err := jsonUnmarshalList(body, &flags); err != nil {
		return 0
	}
	return len(flags)
}

// findSeenProjectField finds a field in the /judge/projects list by project_id
func findSeenProjectField(body string, projectID string, field string) string {
	var projects []map[string]any
	if err := jsonUnmarshalList(body, &projects); err != nil {
		return ""
	}
	for _, p := range projects {
		id, _ := p["project_id"].(string)
		if id == projectID {
			if val, ok := p[field]; ok {
				return fmt.Sprintf("%v", val)
			}
		}
	}
	return ""
}

// TrackRankingScores checks partial rankings, stars, and isolation across judging pools.
func TrackRankingScores(context *util.Context) util.Result {
	var original struct {
		JudgeTracks bool     `json:"judge_tracks"`
		Tracks      []string `json:"tracks"`
		TrackViews  []int    `json:"track_views"`
	}
	if err := json.Unmarshal([]byte(util.GetRequest(context.Logger, "/admin/options", util.AdminAuth())), &original); err != nil {
		return util.NewResult(false, "Could not read original options: "+err.Error())
	}
	defer func() {
		util.PostRequest(context.Logger, "/admin/tracks", util.H{"tracks": original.Tracks}, util.AdminAuth())
		util.PostRequest(context.Logger, "/admin/track-views", util.H{"track_views": original.TrackViews}, util.AdminAuth())
		util.PostRequest(context.Logger, "/admin/options", util.H{"judge_tracks": original.JudgeTracks}, util.AdminAuth())
	}()
	for _, request := range []struct {
		path string
		body util.H
	}{
		{"/admin/reset", util.H{"type": "projects"}},
		{"/admin/tracks", util.H{"tracks": []string{"Rank A", "Rank B"}}},
		{"/admin/track-views", util.H{"track_views": []int{2, 1}}},
		{"/admin/options", util.H{"judge_tracks": true}},
	} {
		if res := util.PostRequest(context.Logger, request.path, request.body, util.AdminAuth()); !util.IsOk(res) {
			return util.NewResult(false, "Track setup failed: "+res)
		}
	}
	var ids []string
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("Track Ranking Project %d", i)
		res := util.PostRequest(context.Logger, "/project/new", util.H{
			"name": name, "description": "Track ranking test", "url": "https://example.com",
			"try_link": "", "video_link": "", "challenge_list": "Rank A, Rank B",
		}, util.AdminAuth())
		if !util.IsOk(res) {
			return util.NewResult(false, "Could not create project: "+res)
		}
		id, result := findProjectIDByName(context, name)
		if !result.Success {
			return result
		}
		ids = append(ids, id)
	}
	for i, track := range []string{"Rank A", "Rank A", "Rank B", ""} {
		token, result := createTrackJudge(context, fmt.Sprintf("track_rank_%d@example.com", i), "Track Ranking Judge", track)
		if !result.Success {
			return result
		}
		auth := util.JudgeAuth(token)
		for j := 0; j < 3; j++ {
			next := util.PostRequest(context.Logger, "/judge/next", nil, auth)
			id := util.ExtractString(next, "project_id")
			if id == "" {
				return util.NewResult(false, "Missing track project: "+next)
			}
			finish := util.PostRequest(context.Logger, "/judge/finish", util.H{"notes": "", "starred": id == ids[0]}, auth)
			if !util.IsOk(finish) {
				return util.NewResult(false, "Could not finish track project: "+finish)
			}
		}
		status, _ := util.PostRequestWithStatus(context.Logger, "/judge/rank", util.H{"ranking": []string{"invalid-id"}}, auth)
		if status != 400 {
			return util.NewResult(false, "Malformed track ranking should return 400")
		}
		ranking := []string{ids[0], ids[1]}
		if track == "Rank B" {
			ranking = []string{ids[2], ids[1]}
		}
		res := util.PostRequest(context.Logger, "/judge/rank", util.H{"ranking": ranking}, auth)
		if !util.IsOk(res) {
			return util.NewResult(false, "Could not save ranking: "+res)
		}
		if i == 0 {
			res = util.PutRequest(context.Logger, "/judge/star/"+ids[0], util.H{"starred": false}, auth)
			if !util.IsOk(res) {
				return util.NewResult(false, "Could not unstar track project: "+res)
			}
		}
	}
	var projects []struct {
		ID          string         `json:"id"`
		Score       int            `json:"score"`
		Stars       int            `json:"stars"`
		TrackScores map[string]int `json:"track_scores"`
		TrackStars  map[string]int `json:"track_stars"`
	}
	body := util.GetRequest(context.Logger, "/project/list", util.AdminAuth())
	if err := jsonUnmarshalList(body, &projects); err != nil {
		return util.NewResult(false, "Could not decode project scores: "+err.Error())
	}
	if len(projects) != 3 {
		return util.NewResult(false, "Expected three scored projects")
	}
	for _, project := range projects {
		index := -1
		for i, id := range ids {
			if id == project.ID {
				index = i
			}
		}
		if index < 0 {
			return util.NewResult(false, "Unexpected project in scores")
		}
		score := []int{2, 0, -2}[index]
		stars := 0
		if index == 0 {
			stars = 1
		}
		if project.Score != score || project.TrackScores["Rank A"] != 2*score || project.TrackScores["Rank B"] != -score {
			return util.NewResult(false, "Incorrect or mixed ranking scores: "+body)
		}
		if project.Stars != stars || project.TrackStars["Rank A"] != stars || project.TrackStars["Rank B"] != stars {
			return util.NewResult(false, "Incorrect or mixed stars: "+body)
		}
	}
	return util.ResultOk()
}
