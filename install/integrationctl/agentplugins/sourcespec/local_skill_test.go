package sourcespec

import "testing"

func TestLocalSkillSerializedPathsAreHostIndependent(t *testing.T) {
	for _, value := range []string{"/home/me/skills/review", "C:/Users/me/skills/review", "//server/share/skills/review", "/space here/review"} {
		if err := ValidateLocalSkillPath(value); err != nil {
			t.Errorf("reject %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "./skills/review", "C:review", `C:\skills\review`, "//server", "//server//review", "/skills/../review", "/skills//review", "/skills/review/", "/skills/\nreview"} {
		if ValidateLocalSkillPath(value) == nil {
			t.Errorf("accepted ambiguous path %q", value)
		}
	}
	key := LocalSkillKey("/skills/review", "review")
	if key == LocalSkillKey("/other/review", "review") || key == LocalSkillKey("/skills/review", "other") {
		t.Fatal("local source identity collapsed distinct paths or names")
	}
}
