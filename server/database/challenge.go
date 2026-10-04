package database

import (
	"context"
	"sort"

	"go.mongodb.org/mongo-driver/mongo"
)

type ChallengeNomination struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Location  int64  `json:"location"`
	Stars     int    `json:"stars"`
}

// GetChallengeNominations summarizes general judges' nominations by challenge.
func GetChallengeNominations(db *mongo.Database, ctx context.Context) (map[string][]ChallengeNomination, error) {
	judges, err := FindAllJudges(db, ctx)
	if err != nil {
		return nil, err
	}
	projects, err := FindAllProjects(db, ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]int)
	for i, project := range projects {
		byID[project.Id.Hex()] = i
	}
	counts := make(map[string]map[string]int)
	for _, judge := range judges {
		if judge.Track != "" {
			continue
		}
		for _, seen := range judge.SeenProjects {
			id := seen.ProjectId.Hex()
			if _, ok := byID[id]; !ok {
				continue
			}
			for _, challenge := range seen.ChallengeStars {
				if counts[challenge] == nil {
					counts[challenge] = make(map[string]int)
				}
				counts[challenge][id]++
			}
		}
	}
	result := make(map[string][]ChallengeNomination)
	for challenge, nominations := range counts {
		rows := make([]ChallengeNomination, 0, len(nominations))
		for id, stars := range nominations {
			project := projects[byID[id]]
			rows = append(rows, ChallengeNomination{ProjectID: id, Name: project.Name, Location: project.Location, Stars: stars})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Stars != rows[j].Stars {
				return rows[i].Stars > rows[j].Stars
			}
			return rows[i].Location < rows[j].Location
		})
		result[challenge] = rows
	}
	return result, nil
}
