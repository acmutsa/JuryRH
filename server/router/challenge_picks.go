package router

import (
	"errors"
	"net/http"
	"server/database"
	"server/judging"
	"server/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type challengePickProject struct {
	ID       primitive.ObjectID `json:"id"`
	Name     string             `json:"name"`
	Location int64              `json:"location"`
	Starred  bool               `json:"starred"`
}

type challengePickGroup struct {
	Name      string                 `json:"name"`
	Judged    int                    `json:"judged"`
	Total     int                    `json:"total"`
	Remaining int                    `json:"remaining"`
	Projects  []challengePickProject `json:"projects"`
}

// GetJudgeChallengePicks lists only eligible judged projects, grouped by enabled challenge.
func GetJudgeChallengePicks(ctx *gin.Context) {
	state := GetState(ctx)
	judge := ctx.MustGet("judge").(*models.Judge)
	options, err := database.GetOptions(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load settings"})
		return
	}
	groups := []challengePickGroup{}
	if judge.Track == "" {
		projects, err := database.FindAllProjects(state.Db, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load eligible projects"})
			return
		}
		seen := make(map[primitive.ObjectID]models.JudgedProject)
		for _, project := range judge.SeenProjects {
			seen[project.ProjectId] = project
		}
		used := judging.ChallengeStarUsage(judge)
		for _, name := range options.OptInChallenges {
			group := challengePickGroup{Name: name, Remaining: max(0, challengeStarLimit()-used[name]), Projects: []challengePickProject{}}
			for _, project := range projects {
				eligible := false
				for _, challenge := range project.ChallengeList {
					if challenge == name {
						eligible = true
					}
				}
				if !eligible {
					continue
				}
				group.Total++
				if judged, ok := seen[project.Id]; ok {
					starred := false
					for _, challenge := range judged.ChallengeStars {
						if challenge == name {
							starred = true
						}
					}
					group.Judged++
					group.Projects = append(group.Projects, challengePickProject{ID: project.Id, Name: project.Name, Location: project.Location, Starred: starred})
				}
			}
			groups = append(groups, group)
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"challenges": groups, "limit": challengeStarLimit(), "locked": options.Deliberation})
}

type challengePickRequest struct {
	Challenge        string             `json:"challenge"`
	ProjectID        primitive.ObjectID `json:"project_id"`
	ReplaceProjectID primitive.ObjectID `json:"replace_project_id"`
	Starred          bool               `json:"starred"`
}

// UpdateJudgeChallengePick revises a judged project's nomination atomically.
func UpdateJudgeChallengePick(ctx *gin.Context) {
	state := GetState(ctx)
	judge := ctx.MustGet("judge").(*models.Judge)
	var request challengePickRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.Challenge == "" || request.ProjectID == primitive.NilObjectID {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge pick request"})
		return
	}
	status := http.StatusInternalServerError
	err := database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		currentJudge, err := database.FindJudge(state.Db, sc, judge.Id)
		if err != nil {
			return err
		}
		if currentJudge == nil {
			return errors.New("judge not found")
		}
		options, err := database.GetOptions(state.Db, sc)
		if err != nil {
			return err
		}
		if options.Deliberation {
			status = http.StatusBadRequest
			return errors.New("challenge picks are locked during deliberation")
		}
		project, err := database.FindProject(state.Db, sc, &request.ProjectID)
		if err != nil {
			return err
		}
		if project == nil {
			status = http.StatusNotFound
			return errors.New("project not found")
		}
		if err := judging.ChangeChallengePick(currentJudge, project, options.OptInChallenges, request.Challenge, request.Starred, request.ReplaceProjectID, challengeStarLimit()); err != nil {
			status = http.StatusBadRequest
			return err
		}
		status = http.StatusInternalServerError
		return database.UpdateJudgeSeenProjects(state.Db, sc, currentJudge)
	})
	if err != nil {
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}
	state.Logger.JudgeLogf(judge, "Updated challenge pick for %s on project %s; starred=%t", request.Challenge, request.ProjectID.Hex(), request.Starred)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1})
}
