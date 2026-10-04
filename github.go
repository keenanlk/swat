package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Issue struct {
	Repo      string    `json:"-"` // owner/name, as GitHub spells it
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	UpdatedAt time.Time `json:"updated_at"`
	RepoURL   string    `json:"repository_url"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (i Issue) key() string { return issueKey(i.Repo, i.Number) }

func (i Issue) labelNames() []string {
	out := make([]string, len(i.Labels))
	for n, l := range i.Labels {
		out[n] = l.Name
	}
	return out
}

type Comment struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

type GitHub struct {
	token  string
	source string // where the token came from, for swat doctor
	http   *http.Client
}

func newGitHub() (*GitHub, error) {
	token, source := os.Getenv("GITHUB_TOKEN"), "GITHUB_TOKEN"
	if token == "" {
		if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
			token, source = strings.TrimSpace(string(out)), "gh auth token"
		}
	}
	if token == "" {
		return nil, fmt.Errorf("no GitHub credentials: run `gh auth login` or set GITHUB_TOKEN")
	}
	return &GitHub{token: token, source: source, http: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (g *GitHub) get(path string, into any) error {
	req, err := http.NewRequest("GET", "https://api.github.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("GitHub: %s %s", resp.Status, e.Message)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func (g *GitHub) login() (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	return u.Login, g.get("/user", &u)
}

// searchQuery is the GitHub search for a user's open issues, narrowed by
// the user's own qualifiers.
func searchQuery(assignee, extra string) string {
	return strings.TrimSpace(fmt.Sprintf("is:issue is:open archived:false assignee:%s %s", assignee, extra))
}

func (g *GitHub) searchIssues(query string) ([]Issue, error) {
	var res struct {
		Items []Issue `json:"items"`
	}
	if err := g.get("/search/issues?per_page=100&sort=updated&q="+url.QueryEscape(query), &res); err != nil {
		return nil, err
	}
	for i := range res.Items {
		res.Items[i].Repo = strings.TrimPrefix(res.Items[i].RepoURL, "https://api.github.com/repos/")
	}
	return res.Items, nil
}

func (g *GitHub) comments(repo string, number int) ([]Comment, error) {
	var cs []Comment
	return cs, g.get(fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, number), &cs)
}
