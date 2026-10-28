package cards

import (
	"strings"
	"testing"
)

func TestOracleRoleTagsInheritAncestorsAndMultipleRoles(t *testing.T) {
	payload := `{"id":"tutor","slug":"tutor","taggings":[]}
 {"id":"ramp","slug":"ramp","taggings":[]}
 {"id":"land","slug":"tutor-land-to-battlefield","parent_ids":["tutor","ramp"],"taggings":[{"oracle_id":"one"}]}
 {"id":"creature","slug":"tutor-creature","parent_ids":["tutor"],"taggings":[{"oracle_id":"two"}]}
 {"id":"cycle1","slug":"a","parent_ids":["cycle2"]}
 {"id":"cycle2","slug":"b","parent_ids":["cycle1"]}`
	roles, err := decodeOracleRoleTags(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(roles["one"], ",") != "ramp,tutor" || strings.Join(roles["two"], ",") != "tutor" {
		t.Fatalf("lost inherited roles: %#v", roles)
	}
	for _, bad := range []string{"[]", `{"id":"one","slug":"irrelevant"}`, `{"id":`} {
		if _, err := decodeOracleRoleTags(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted invalid or empty role snapshot: %s", bad)
		}
	}
}
