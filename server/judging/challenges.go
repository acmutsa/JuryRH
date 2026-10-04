package judging

import (
	"fmt"
	"server/models"
)

// ChallengeStarUsage counts a general judge's nominations in each challenge.
func ChallengeStarUsage(judge *models.Judge) map[string]int {
	used := make(map[string]int)
	for _, project := range judge.SeenProjects {
		for _, challenge := range project.ChallengeStars {
			used[challenge]++
		}
	}
	return used
}

// ValidateChallengeStars checks project eligibility, opt-in status, and the judge's quota.
func ValidateChallengeStars(judge *models.Judge, project *models.Project, enabled []string, selected []string, limit int) error {
	if len(selected) == 0 {
		return nil
	}
	if judge.Track != "" {
		return fmt.Errorf("challenge nominations are only available during general judging")
	}
	allowed := make(map[string]bool)
	for _, challenge := range enabled {
		allowed[challenge] = true
	}
	eligible := make(map[string]bool)
	for _, challenge := range project.ChallengeList {
		eligible[challenge] = true
	}
	used := ChallengeStarUsage(judge)
	selectedOnce := make(map[string]bool)
	for _, challenge := range selected {
		if selectedOnce[challenge] || !allowed[challenge] || !eligible[challenge] {
			return fmt.Errorf("invalid challenge nomination: %s", challenge)
		}
		selectedOnce[challenge] = true
		if used[challenge] >= limit {
			return fmt.Errorf("challenge nomination limit reached for %s", challenge)
		}
	}
	return nil
}
