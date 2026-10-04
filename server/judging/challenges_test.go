package judging

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"reflect"
	"server/models"
	"testing"
)

func TestValidateChallengeStars(t *testing.T) {
	project := models.NewProject("Candidate", 1, 0, "", "", "", "", []string{"Climate", "Health"})
	judge := models.NewJudge("Judge", "", "", 0)
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

func TestChangeChallengePick(t *testing.T) {
	old := models.NewProject("Old", 1, 0, "", "", "", "", []string{"Climate"})
	other := models.NewProject("Other", 2, 0, "", "", "", "", []string{"Climate"})
	target := models.NewProject("Better", 3, 0, "", "", "", "", []string{"Climate"})
	old.Id, other.Id, target.Id = primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	fresh := func() *models.Judge {
		judge := models.NewJudge("Judge", "", "", 0)
		judge.SeenProjects = []models.JudgedProject{
			{ProjectId: old.Id, Starred: true, ChallengeStars: []string{"Climate", "Health"}},
			{ProjectId: other.Id, ChallengeStars: []string{"Climate"}},
			{ProjectId: target.Id, ChallengeStars: []string{}},
		}
		return judge
	}
	t.Run("replace at quota without affecting general or other challenge stars", func(t *testing.T) {
		judge := fresh()
		if err := ChangeChallengePick(judge, target, []string{"Climate"}, "Climate", true, old.Id, 2); err != nil {
			t.Fatal(err)
		}
		if ChallengeStarUsage(judge)["Climate"] != 2 || !judge.SeenProjects[0].Starred || !reflect.DeepEqual(judge.SeenProjects[0].ChallengeStars, []string{"Health"}) || !reflect.DeepEqual(judge.SeenProjects[2].ChallengeStars, []string{"Climate"}) {
			t.Fatal("replacement changed unrelated ratings or failed to move the star")
		}
	})
	cases := []struct {
		name     string
		replace  primitive.ObjectID
		track    string
		enabled  []string
		eligible []string
	}{
		{"quota without replacement", primitive.NilObjectID, "", []string{"Climate"}, []string{"Climate"}},
		{"invalid previous pick", target.Id, "", []string{"Climate"}, []string{"Climate"}},
		{"disabled challenge", old.Id, "", []string{}, []string{"Climate"}},
		{"track judge", old.Id, "Design", []string{"Climate"}, []string{"Climate"}},
		{"ineligible target", old.Id, "", []string{"Climate"}, []string{"Health"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			judge := fresh()
			judge.Track = tc.track
			before := append([]models.JudgedProject{}, judge.SeenProjects...)
			project := *target
			project.ChallengeList = tc.eligible
			if err := ChangeChallengePick(judge, &project, tc.enabled, "Climate", true, tc.replace, 2); err == nil {
				t.Fatal("invalid revision accepted")
			}
			if !reflect.DeepEqual(before, judge.SeenProjects) {
				t.Fatal("failed revision changed the old picks")
			}
		})
	}
	t.Run("unjudged target", func(t *testing.T) {
		judge := fresh()
		judge.SeenProjects = judge.SeenProjects[:2]
		if err := ChangeChallengePick(judge, target, []string{"Climate"}, "Climate", true, old.Id, 2); err == nil {
			t.Fatal("unjudged target accepted")
		}
	})
	t.Run("remove frees allowance", func(t *testing.T) {
		judge := fresh()
		if err := ChangeChallengePick(judge, old, []string{"Climate"}, "Climate", false, primitive.NilObjectID, 2); err != nil {
			t.Fatal(err)
		}
		if ChallengeStarUsage(judge)["Climate"] != 1 {
			t.Fatal("star not removed")
		}
	})
}
