package router

import (
	"errors"
	"net/http"
	"server/config"
	"server/database"
	"server/funcs"
	"server/judging"
	"server/models"
	"server/util"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GET /judge - Endpoint to get the judge from the token
func GetJudge(ctx *gin.Context) {
	// Get the judge from the context (See middleware.go)
	judge := ctx.MustGet("judge").(*models.Judge)

	// Send Judge
	ctx.JSON(http.StatusOK, judge)
}

// POST /judge/new - Endpoint to add a single judge
func AddJudge(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the request
	var judgeReq models.AddJudgeRequest
	err := ctx.BindJSON(&judgeReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Make sure all required request fields are defined
	if judgeReq.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var judge *models.Judge

	// Run remaining actions in a transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Determine group judge should go in
		group := int64(0)
		if judgeReq.Track == "" {
			group, err = database.GetMinJudgeGroup(state.Db, sc)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting judge group: " + err.Error()})
				return err
			}
		}

		// Create the judge
		judge = models.NewJudge(judgeReq.Name, judgeReq.Track, judgeReq.Notes, group)

		// Insert the judge into the database
		err = database.InsertJudge(state.Db, sc, judge)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	track := judge.Track
	if track == "" {
		track = "general"
	}
	state.Logger.AdminLogf("Added judge %s, track: %s", judge.Name, track)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/login - Code entry has been replaced by QR registration.
func JudgeCodeLoginRemoved(ctx *gin.Context) {
	ctx.JSON(http.StatusGone, gin.H{"error": "judge code login has been removed; scan the organizer's QR code"})
}

// POST /judge/auth - Check to make sure a judge is authenticated
func JudgeAuthenticated(ctx *gin.Context) {
	// This route will run the middleware first, and if the middleware
	// passes, then that means the judge is authenticated

	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/csv - Endpoint to add judges from a CSV file
func AddJudgesCsv(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the CSV file from the request
	file, err := ctx.FormFile("csv")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading CSV file from request: " + err.Error()})
		return
	}

	// Get the form fields from the request
	hasHeader := ctx.PostForm("hasHeader") == "true"

	// Open the file
	f, err := file.Open()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error opening CSV file: " + err.Error()})
		return
	}

	// Read the file
	content := make([]byte, file.Size)
	_, err = f.Read(content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error reading CSV file: " + err.Error()})
		return
	}

	// Parse the CSV file
	judges, err := funcs.ParseJudgeCSV(string(content), hasHeader)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error parsing CSV file: " + err.Error()})
		return
	}

	// Do the remaining actions in a transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Determine group numbers for the new judges
		groups, err := database.GetNextNJudgeGroups(state.Db, sc, len(judges), false)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting judge groups: " + err.Error()})
			return err
		}

		// Assign group numbers to the new judges
		for i, judge := range judges {
			judge.Group = groups[i]
		}

		// Insert judges into the database
		err = database.InsertJudges(state.Db, sc, judges)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error inserting judges into database: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	state.Logger.AdminLogf("Added %d judges from CSV", len(judges))
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// GET /judge/welcome - Endpoint to check if a judge has read the welcome message
func CheckJudgeReadWelcome(ctx *gin.Context) {
	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Send OK
	if judge.ReadWelcome {
		ctx.JSON(http.StatusOK, gin.H{"ok": 1})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"ok": 0})
	}
}

// POST /judge/welcome - Endpoint to set a judge's readWelcome field to true
func SetJudgeReadWelcome(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Update judge in database
	err := database.UpdateJudgeReadWelcome(state.Db, ctx, &judge.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error updating judge in database: " + err.Error()})
		return
	}

	// Send OK
	state.Logger.JudgeLogf(judge, "Read welcome message")
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// GET /judge/list - Endpoint to get a list of all judges
func ListJudges(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get judges from database
	judges, err := database.FindAllJudges(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error finding judges in database: " + err.Error()})
		return
	}

	// Send OK
	ctx.JSON(http.StatusOK, judges)
}

// GET /judge/stats - Endpoint to get stats about the judges
func JudgeStats(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Aggregate judge stats
	stats, err := database.AggregateJudgeStats(state.Db)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error aggregating judge stats: " + err.Error()})
		return
	}

	// Send OK
	ctx.JSON(http.StatusOK, stats)
}

// DELETE /judge/:id - Endpoint to delete a judge
func DeleteJudge(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge ID from the URL
	judgeId := ctx.Param("id")

	// Convert judge ID string to ObjectID
	judgeObjectId, err := primitive.ObjectIDFromHex(judgeId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judge ID"})
		return
	}

	// Run in transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get judge data
		judge, err := database.FindJudge(state.Db, sc, judgeObjectId)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not get judge: " + err.Error()})
			return err
		}

		// Delete judge from database
		err = database.DeleteJudgeById(state.Db, sc, judgeObjectId)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error deleting judge from database: " + err.Error()})
			return err
		}

		// Decrement the seen number of all projects in the judge's seen_projects array
		err = database.DecrementDeletedJudgeSeen(state.Db, sc, judge)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not decrement seen for judged projects: " + err.Error()})
			return err
		}

		// Delete all flags for judge
		err = database.DeleteFlagsCascade(state.Db, sc, nil, &judgeObjectId)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error deleting flags for judge: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	state.Logger.AdminLogf("Deleted judge %s", judgeId)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/next - Endpoint to get the next project for a judge
func GetNextJudgeProject(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// If the judge already has a next project, return that project
	if judge.Current != nil {
		ctx.JSON(http.StatusOK, gin.H{"project_id": judge.Current.Hex()})
		return
	}

	// Otherwise, get the next project for the judge
	err := database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get options
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			return errors.New("error getting options: " + err.Error())
		}

		// If the clock is paused, return an empty object
		// This is to ensure that no projects are gotten if the clock is paused
		state.Clock.Mutex.Lock()
		if !state.Clock.State.Running || options.Deliberation {
			return nil
		}
		state.Clock.Mutex.Unlock()

		project, err := judging.PickNextProject(state.Db, sc, judge, state.Comps)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error picking next project: " + err.Error()})
			return nil
		}

		// If there is no next project, return an empty object
		if project == nil {
			ctx.JSON(http.StatusOK, gin.H{})
			return nil
		}

		// Update judge and project
		err = database.UpdateAfterPicked(state.Db, sc, project, judge)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error updating next project in database: " + err.Error()})
			return nil
		}

		// Send OK and project ID
		state.Logger.JudgeLogf(judge, "Picked new project %s (%s)", project.Name, project.Id.Hex())
		ctx.JSON(http.StatusOK, gin.H{"project_id": project.Id.Hex()})
		return nil
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

// GET /judge/projects - Endpoint to get a list of projects that a judge has seen
func GetJudgeProjects(ctx *gin.Context) {
	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Return the judge's seen projects list
	ctx.JSON(http.StatusOK, judge.SeenProjects)
}

type JudgedProjectWithUrl struct {
	models.JudgedProject
	Url string `bson:"url" json:"url"`
}

func addUrlToJudgedProject(project *models.JudgedProject, url string) *JudgedProjectWithUrl {
	return &JudgedProjectWithUrl{
		JudgedProject: models.JudgedProject{
			ProjectId:   project.ProjectId,
			Name:        project.Name,
			Location:    project.Location,
			Description: project.Description,
			Notes:       project.Notes,
			Starred:     project.Starred,
		},
		Url: url,
	}
}

// GET /judge/project/:id - Gets a project that's been judged by ID
func GetJudgedProject(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the project ID from the URL
	projectId := ctx.Param("id")

	// Search through the judge seen projects for the project ID
	for _, p := range judge.SeenProjects {
		if p.ProjectId.Hex() == projectId {
			// Add URL to judged project
			proj, err := database.FindProject(state.Db, ctx, &p.ProjectId)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting project url: " + err.Error()})
				return
			}
			jpWithUrl := addUrlToJudgedProject(&p, proj.Url)

			// Parse and send JSON
			ctx.JSON(http.StatusOK, jpWithUrl)
			return
		}
	}

	// Send bad request bc project ID invalid
	ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
}

type SkipRequest struct {
	Reason string `json:"reason"`
}

// POST /judge/skip - Endpoint to skip a project
func JudgeSkip(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the skip reason from the request
	var skipReq SkipRequest
	err := ctx.BindJSON(&skipReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Skipped project ID
	id := judge.Current.Hex()

	// Run remaining actions in a transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Check if judging is paused
		options, err := database.GetOptions(state.Db, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}
		state.Clock.Mutex.Lock()
		newProj := state.Clock.State.Running && !options.Deliberation
		state.Clock.Mutex.Unlock()

		// Skip the project
		err = judging.SkipCurrentProjectWithTx(state.Db, sc, judge, state.Comps, skipReq.Reason, newProj)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	if skipReq.Reason == "busy" {
		state.Logger.JudgeLogf(judge, "Skipped busy project %s", id)
	} else {
		state.Logger.JudgeLogf(judge, "Flagged project %s due to %s", id, skipReq.Reason)
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/hide/:id - Endpoint to hide a judge
func HideJudge(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get ID from URL
	id := ctx.Param("id")

	// Get ID from body
	var hideReq util.HideRequest
	err := ctx.BindJSON(&hideReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Convert ID string to ObjectID
	judgeObjectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judge ID"})
		return
	}

	// Hide judge in database
	err = database.SetJudgeActive(state.Db, &judgeObjectId, !hideReq.Hide)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error hiding judge in database: " + err.Error()})
		return
	}

	// Send OK
	action := "Unhid"
	if hideReq.Hide {
		action = "Hid"
	}
	state.Logger.AdminLogf("%s judge %s", action, id)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/hide - Endpoint to hide selected judges
func HideSelectedJudges(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the request object
	var hideReq util.HideSelectedRequest
	err := ctx.BindJSON(&hideReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Hide judges in database
	err = database.SetJudgesActive(state.Db, hideReq.Items, !hideReq.Hide)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error hiding judges in database: " + err.Error()})
		return
	}

	// Send OK
	action := "Unhid"
	if hideReq.Hide {
		action = "Hid"
	}
	state.Logger.AdminLogf("%s %d judges", action, len(hideReq.Items))
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// PUT /judge/:id - Endpoint to edit a judge
func EditJudge(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the body content
	var judgeReq models.AddJudgeRequest
	err := ctx.BindJSON(&judgeReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Get the judge ID from the path
	judgeId := ctx.Param("id")

	// Convert ID string to ObjectID
	judgeObjectId, err := primitive.ObjectIDFromHex(judgeId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judge ID"})
		return
	}

	// Edit judge in database
	err = database.UpdateJudgeBasicInfo(state.Db, &judgeObjectId, &judgeReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error editing judge in database: " + err.Error()})
		return
	}

	// Send OK
	state.Logger.AdminLogf("Edited judge %s: %s", judgeId, util.StructToString(judgeReq))
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

type JudgeScoreRequest struct {
	Notes          string   `json:"notes"`
	Starred        bool     `json:"starred"`
	ChallengeStars []string `json:"challenge_stars"`
}

func challengeStarLimit() int {
	limit, err := strconv.Atoi(config.GetOptEnv("JURY_CHALLENGE_STAR_LIMIT", "2"))
	if err != nil || limit < 1 {
		return 2
	}
	return limit
}

// GET /judge/challenges - Eligible opt-in challenges and remaining nominations.
func GetJudgeChallenges(ctx *gin.Context) {
	state := GetState(ctx)
	judge := ctx.MustGet("judge").(*models.Judge)
	result := gin.H{"challenges": []string{}, "remaining": map[string]int{}, "limit": challengeStarLimit(), "message": "Challenge stars are available when judging a project."}
	if judge.Track != "" {
		result["message"] = "Challenge stars are available during general judging. Your stars here count for your assigned track."
	}
	if judge.Track == "" && judge.Current != nil {
		options, err := database.GetOptions(state.Db, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		project, err := database.FindProject(state.Db, ctx, judge.Current)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if project == nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "current project not found"})
			return
		}
		eligible := make(map[string]bool)
		for _, challenge := range project.ChallengeList {
			eligible[challenge] = true
		}
		used := judging.ChallengeStarUsage(judge)
		challenges := make([]string, 0)
		remaining := make(map[string]int)
		for _, challenge := range options.OptInChallenges {
			if eligible[challenge] {
				challenges = append(challenges, challenge)
				remaining[challenge] = max(0, challengeStarLimit()-used[challenge])
			}
		}
		result["challenges"] = challenges
		result["remaining"] = remaining
		if len(options.OptInChallenges) == 0 {
			result["message"] = "No challenges are enabled for stars. Ask an organizer to enable them in Settings → Challenge Stars."
		} else if len(challenges) == 0 {
			result["message"] = "This project has not entered any challenges enabled for stars."
		} else {
			result["message"] = ""
		}
	}
	ctx.JSON(http.StatusOK, result)
}

// POST /judge/finish - Endpoint to finish judging a project
func JudgeFinish(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the request object
	var scoreReq JudgeScoreRequest
	err := ctx.BindJSON(&scoreReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Run remaining actions in a transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		currentJudge, err := database.FindJudge(state.Db, sc, judge.Id)
		if err != nil || currentJudge == nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error loading judge"})
			return errors.New("error loading judge")
		}
		if currentJudge.Current == nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "judge has no current project"})
			return errors.New("judge has no current project")
		}
		// Get the options and return error if deliberations
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}
		if options.Deliberation {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cannot score due to deliberation mode being enabled"})
			return errors.New("deliberation mode enabled")
		}

		// Get the project from the database
		project, err := database.FindProject(state.Db, sc, currentJudge.Current)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error finding project in database: " + err.Error()})
			return err
		}
		if project == nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "current project not found"})
			return errors.New("current project not found")
		}

		// Create the judged project object
		judgedProject := models.JudgeProjectFromProject(project, scoreReq.Notes, scoreReq.Starred)
		if err := judging.ValidateChallengeStars(currentJudge, project, options.OptInChallenges, scoreReq.ChallengeStars, challengeStarLimit()); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return err
		}
		judgedProject.ChallengeStars = scoreReq.ChallengeStars

		// If groups are enabled and auto switch, move the judge to the next group conditionally
		if options.MultiGroup && options.SwitchingMode == "auto" {
			err = judging.MoveJudgeGroup(state.Db, sc, currentJudge, options)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error moving judge group: " + err.Error()})
				return err
			}
		}

		// Update the judge and project
		err = database.UpdateAfterSeen(state.Db, sc, currentJudge, judgedProject)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error storing scores in database: " + err.Error()})
			return err
		}

		// Reset list of skipped projects due to busy status
		err = database.ResetBusyProjectListForJudge(state.Db, sc, currentJudge)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error resetting busy project list in database: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	starred := ""
	if scoreReq.Starred {
		starred = " and starred project"
	}
	projId := judge.Current.Hex()
	state.Logger.JudgeLogf(judge, "Finished judging project %s%s; nominated for %v", projId, starred, scoreReq.ChallengeStars)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

type RankRequest struct {
	Ranking []primitive.ObjectID `json:"ranking"`
}

// POST /judge/rank - Update the judge's ranking of projects
func JudgeRank(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the request object
	var rankReq RankRequest
	err := ctx.BindJSON(&rankReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Wrap in transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get the options and return error if deliberations
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}
		if options.Deliberation {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cannot score due to deliberation mode being enabled"})
			return err
		}

		// Calculate updated rankings
		judge.Rankings = rankReq.Ranking
		agg := judging.AggregateRanking(judge)

		// Update the judge's ranking
		err = database.UpdateJudgeRanking(state.Db, sc, judge.Id, judge.Rankings, agg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error updating judge ranking in database: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	oldRanks := util.RankingToString(judge.Rankings)
	state.Logger.JudgeLogf(judge, "Updated rankings from %s to %s", oldRanks, util.RankingToString(rankReq.Ranking))
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

type JudgeStarRequest struct {
	Starred bool `json:"starred"`
}

// PUT /judge/star/:id - Update the judge's star status on a judged project
func JudgeStar(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the project id from the URL
	rawProjectId := ctx.Param("id")
	projectId, err := primitive.ObjectIDFromHex(rawProjectId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	// Get the request object
	var starReq JudgeStarRequest
	err = ctx.BindJSON(&starReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Wrap in transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get the options and return error if deliberations
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}
		if options.Deliberation {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cannot score due to deliberation mode being enabled"})
			return err
		}

		// If the project isn't in the judge's seen projects, return an error
		index := util.FindSeenProjectIndex(judge, projectId)
		if index == -1 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "judge hasn't seen project or project is invalid"})
			return err
		}

		// Update the judge's object for the project
		err = database.UpdateJudgeStars(state.Db, sc, judge.Id, index, starReq.Starred)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error updating judge starred in database: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	starred := "Removed"
	if starReq.Starred {
		starred = "Added"
	}
	state.Logger.JudgeLogf(judge, "%s star for project %s", starred, rawProjectId)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

type UpdateNotesRequest struct {
	Notes string `json:"notes"`
}

// PUT /judge/notes/:id - Update the notes of a judge
func JudgeUpdateNotes(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the judge from the context
	judge := ctx.MustGet("judge").(*models.Judge)

	// Get the project ID from the URL
	rawProjectId := ctx.Param("id")
	projectId, err := primitive.ObjectIDFromHex(rawProjectId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	// Get the request object
	var notesReq UpdateNotesRequest
	err = ctx.BindJSON(&notesReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// If the project isn't in the judge's seen projects, return an error
	index := util.FindSeenProjectIndex(judge, projectId)
	if index == -1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "judge hasn't seen project or project is invalid"})
		return
	}

	// Update that specific index of the seen projects array
	judge.SeenProjects[index].Notes = notesReq.Notes

	// Update the judge's object for the project
	err = database.UpdateJudgeSeenProjects(state.Db, ctx, judge)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error updating judge score in database: " + err.Error()})
		return
	}

	// Send OK
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/reassign - Reassigns judge numbers to all judges that are not in a track
func ReassignJudgeGroups(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Run the remaining actions in a transaction
	err := database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get options from database
		options, err := database.GetOptions(state.Db, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}

		// Don't do if not enabled
		if !options.MultiGroup {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "multi-group not enabled"})
			return errors.New("multi-group not enabled")
		}

		// Reassign judge groups
		err = database.PutJudgesInGroups(state.Db)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error reassigning judge groups: " + err.Error()})
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	// Send OK
	state.Logger.AdminLogf("Reassigned judge groups")
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// PUT /judge/move/:id - Move a judge to a different group
func MoveJudge(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the ID from the URL
	id := ctx.Param("id")

	// Get the request object
	var moveReq util.MoveGroupRequest
	err := ctx.BindJSON(&moveReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Convert ID string to ObjectID
	judgeObjectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judge ID"})
		return
	}

	// Move the judge to the new group
	err = database.SetJudgeGroup(state.Db, ctx, &judgeObjectId, moveReq.Group)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error moving judge group: " + err.Error()})
		return
	}

	// Send OK
	state.Logger.AdminLogf("Moved judge %s to group %d", id, moveReq.Group)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

// POST /judge/move - Move selected judges to a different group
func MoveSelectedJudges(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the request object
	var moveReq util.MoveSelectedRequest
	err := ctx.BindJSON(&moveReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	// Move the judges to the new group
	err = database.SetJudgesGroup(state.Db, ctx, moveReq.Items, moveReq.Group)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error moving judges group: " + err.Error()})
		return
	}

	// Send OK
	state.Logger.AdminLogf("Moved %d judges to group %d", len(moveReq.Items), moveReq.Group)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}

type AddJudgeFromQRRequest struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Track string `json:"track"`
}

// POST /qr/add - Register and authenticate a judge from a QR code
func AddJudgeFromQR(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the request object
	var qrReq AddJudgeFromQRRequest
	err := ctx.BindJSON(&qrReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error reading request body: " + err.Error()})
		return
	}

	qrReq.Name = strings.TrimSpace(qrReq.Name)
	if qrReq.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var judge *models.Judge

	// Run the remaining actions in a transaction
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		// Get the options from the database
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
			return err
		}

		// Make sure the code is correct and reject empty code
		expectedCode := options.QRCode
		if qrReq.Track != "" {
			if !options.JudgeTracks || !slices.Contains(options.Tracks, qrReq.Track) {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judging track"})
				return errors.New("invalid judging track")
			}
			expectedCode = options.TrackQRCodes[qrReq.Track]
		}
		if qrReq.Code == "" || expectedCode == "" || qrReq.Code != expectedCode {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid QR code"})
			return errors.New("invalid QR code")
		}

		// Determine group judge should go in
		group := int64(0)
		if qrReq.Track == "" {
			group, err = database.GetMinJudgeGroup(state.Db, sc)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting judge group: " + err.Error()})
				return err
			}
		}

		// Create the judge
		judge = models.NewJudge(qrReq.Name, qrReq.Track, "", group)

		judge.Token, err = util.GenerateToken()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error generating judge token"})
			return err
		}

		// Insert the judge into the database
		err = database.InsertJudge(state.Db, sc, judge)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error inserting judge into database: " + err.Error()})
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Guard against early transaction exit leaving judge nil
	if judge == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error creating judge"})
		return
	}

	// Send OK
	track := ""
	if qrReq.Track != "" {
		track = ", track " + qrReq.Track
	}
	state.Logger.AdminLogf("Added judge %s from QR code%s", judge.Name, track)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1, "token": judge.Token})
}

// GET /judge/deliberation - Get the status of the deliberation
func GetDeliberationStatus(ctx *gin.Context) {
	// Get the state from the context
	state := GetState(ctx)

	// Get the options from the database
	options, err := database.GetOptions(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
		return
	}

	if options.Deliberation {
		ctx.JSON(http.StatusOK, gin.H{"ok": 1})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"ok": 0})
	}
}
