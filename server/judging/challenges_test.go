package judging

import (
	"server/models"
	"testing"
)

func TestValidateChallengeStars(t *testing.T) {
	project := models.NewProject("Candidate", 1, 0, "", "", "", "", []string{"Climate", "Health"})
	judge := models.NewJudge("Judge", "judge@example.com", "", "", 0)
	judge.SeenProjects = []models.JudgedProject{
		{ChallengeStars: []string{"Climate"}},
		{ChallengeStars: []string{"Climate"}},
	}
	enabled := []string{"Climate", "Health"}
	cases := []struct {
		name     string
		selected []string
		valid    bool
	}{
		{"empty", nil, true},
		{"eligible", []string{"Health"}, true},
		{"quota", []string{"Climate"}, false},
		{"disabled", []string{"Other"}, false},
		{"duplicate", []string{"Health", "Health"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChallengeStars(judge, project, enabled, tc.selected, 2)
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v, error = %v", tc.valid, err)
			}
		})
	}
	judge.Track = "Climate"
	if err := ValidateChallengeStars(judge, project, enabled, []string{"Health"}, 2); err == nil {
		t.Fatal("track judge could nominate a challenge")
	}
}
