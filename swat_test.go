package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:Acme/API.git":       "acme/api",
		"https://github.com/acme/api":       "acme/api",
		"https://github.com/acme/api.git":   "acme/api",
		"https://github.com/acme/api/":      "acme/api",
		"ssh://git@github.com/acme/api.git": "acme/api",
		"git@gitlab.com:acme/api.git":       "",
		"https://github.com/acme":           "",
		"https://github.com/acme/api/extra": "",
		"https://example.com/acme/api.git":  "",
	}
	for in, want := range cases {
		if got := parseGitHubURL(in); got != want {
			t.Errorf("parseGitHubURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRankClones(t *testing.T) {
	clones := map[string]Clone{
		"other/thing":   {Name: "other/thing"},
		"acme/web":      {Name: "acme/web"},
		"acme/api":      {Name: "acme/api", Pinned: true},
		"acme/backend":  {Name: "acme/backend", Project: ProjectConfig{IssueRepos: []string{"Acme/Issues"}}},
		"acme/issues":   {Name: "acme/issues"},
		"acme/zzz-docs": {Name: "acme/zzz-docs"},
	}
	var got []string
	for _, c := range rankClones(clones, "Acme/Issues") {
		got = append(got, c.Name)
	}
	want := "acme/issues acme/backend acme/api acme/web acme/zzz-docs other/thing"
	if strings.Join(got, " ") != want {
		t.Errorf("rankClones = %v\nwant %s", got, want)
	}
}

func TestIssueKeyIsCaseInsensitive(t *testing.T) {
	if issueKey("CoSchedule/Issues", 7) != issueKey("coschedule/issues", 7) {
		t.Error("issue keys should ignore repo case")
	}
}

func TestBuildPrompt(t *testing.T) {
	issue := Issue{Repo: "acme/issues", Number: 12, Title: "Dates look wrong", Body: "They show raw ISO strings."}
	clone := Clone{Name: "acme/api", Project: ProjectConfig{Instructions: "Run `make test` before committing."}}
	for _, pause := range []bool{true, false} {
		p := buildPrompt(issue, []Comment{{Body: "Also on mobile."}}, clone, pause)
		for _, want := range []string{"acme/issues#12", "acme/api", "They show raw ISO strings.", "Also on mobile.", stateDir + "/status.json", "make test"} {
			if !strings.Contains(p, want) {
				t.Errorf("pause=%v: prompt is missing %q", pause, want)
			}
		}
		if got := strings.Contains(p, "awaiting_approval and stop"); got != pause {
			t.Errorf("pause=%v: stop-after-plan instruction present = %v", pause, got)
		}
	}
}

func TestSearchQuery(t *testing.T) {
	if got := searchQuery("me", ""); got != "is:issue is:open archived:false assignee:me" {
		t.Errorf("searchQuery = %q", got)
	}
	if got := searchQuery("me", "org:acme"); !strings.HasSuffix(got, " org:acme") {
		t.Errorf("searchQuery with extra = %q", got)
	}
}

func TestStripJSONComments(t *testing.T) {
	in := `{
  // a comment
  "a": "keep // this", /* block
  comment */ "b": "and \"/* this */\""
}`
	var got map[string]string
	if err := json.Unmarshal(stripJSONComments([]byte(in)), &got); err != nil {
		t.Fatal(err)
	}
	if got["a"] != "keep // this" || got["b"] != `and "/* this */"` {
		t.Errorf("got %v", got)
	}
}

func TestStarterConfigIsValidAndEmpty(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(stripJSONComments([]byte(starterConfig())), &m); err != nil {
		t.Fatalf("starter config doesn't parse: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("starter config should set nothing, got %v", m)
	}
	for _, s := range settings {
		if !strings.Contains(starterConfig(), `"`+s.Key+`"`) {
			t.Errorf("starter config doesn't mention %s", s.Key)
		}
	}
}

func TestEveryHelpTopicHasADoc(t *testing.T) {
	for _, tp := range topics {
		if strings.TrimSpace(docText(tp.file)) == "" {
			t.Errorf("topic %s: docs/%s is missing or empty", tp.name, tp.file)
		}
	}
}

func TestDocsCoverEverySetting(t *testing.T) {
	doc := docText("configuration.md")
	for _, s := range settings {
		if !strings.Contains(doc, "### `"+s.Key+"`") {
			t.Errorf("docs/configuration.md has no section for %s", s.Key)
		}
	}
}
