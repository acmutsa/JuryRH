package judging

import (
	"fmt"
	"server/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

// RemoveChallengePick releases a nomination only from a project this judge starred.
func RemoveChallengePick(judge *models.Judge, challenge string, id primitive.ObjectID) error {
	for i := range judge.SeenProjects {
		seen := &judge.SeenProjects[i]
		if seen.ProjectId != id {
			continue
		}
		for j, name := range seen.ChallengeStars {
			if name == challenge {
				seen.ChallengeStars = append(append([]string{}, seen.ChallengeStars[:j]...), seen.ChallengeStars[j+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("previous pick is no longer starred for %s; refresh your picks", challenge)
}

// ChangeChallengePick validates a revision before changing any of the judge's picks.
func ChangeChallengePick(judge *models.Judge, project *models.Project, enabled []string, challenge string, starred bool, replace primitive.ObjectID, limit int) error {
	if judge.Track != "" {
		return fmt.Errorf("challenge picks are only available during general judging")
	}
	allowed := false
	for _, name := range enabled {
		if name == challenge {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("challenge is not enabled for stars")
	}
	candidate := *judge
	candidate.SeenProjects = append([]models.JudgedProject{}, judge.SeenProjects...)
	index := -1
	for i, seen := range candidate.SeenProjects {
		if seen.ProjectId == project.Id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("judge has not judged this project")
	}
	wasStarred := false
	for _, name := range candidate.SeenProjects[index].ChallengeStars {
		if name == challenge {
			wasStarred = true
		}
	}
	if !starred && replace != primitive.NilObjectID {
		return fmt.Errorf("cannot replace a pick when removing a star")
	}
	if replace == project.Id {
		return fmt.Errorf("choose a different previous pick")
	}
	if wasStarred {
		if replace != primitive.NilObjectID {
			return fmt.Errorf("project is already starred")
		}
		if err := RemoveChallengePick(&candidate, challenge, project.Id); err != nil {
			return err
		}
	}
	if starred {
		if replace != primitive.NilObjectID {
			if err := RemoveChallengePick(&candidate, challenge, replace); err != nil {
				return err
			}
		}
		if err := ValidateChallengeStars(&candidate, project, enabled, []string{challenge}, limit); err != nil {
			return err
		}
		candidate.SeenProjects[index].ChallengeStars = append(append([]string{}, candidate.SeenProjects[index].ChallengeStars...), challenge)
	}
	judge.SeenProjects = candidate.SeenProjects
	return nil
}
