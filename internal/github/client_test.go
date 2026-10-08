package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseProjectURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		owner     string
		ownerType OwnerType
		number    int
		wantError bool
	}{
		{name: "organization project", url: "https://github.com/orgs/acme/projects/17", owner: "acme", ownerType: OrganizationOwner, number: 17},
		{name: "user project", url: "https://github.com/users/octocat/projects/2?tab=items", owner: "octocat", ownerType: UserOwner, number: 2},
		{name: "owner shorthand", url: "https://github.com/acme/projects/5", owner: "acme", number: 5},
		{name: "repository URL is not a project", url: "https://github.com/acme/repo", wantError: true},
		{name: "zero project number", url: "https://github.com/orgs/acme/projects/0", wantError: true},
		{name: "other host", url: "https://example.com/orgs/acme/projects/1", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseProjectURL(test.url)
			if test.wantError {
				if err == nil {
					t.Fatal("ParseProjectURL() expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProjectURL() error = %v", err)
			}
			if got.Owner != test.owner || got.OwnerType != test.ownerType || got.Number != test.number {
				t.Fatalf("ParseProjectURL() = %+v, want owner %q, owner type %q, and number %d",
					got, test.owner, test.ownerType, test.number)
			}
		})
	}
}

func TestTokenFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		ghToken     string
		githubToken string
		want        string
	}{
		{name: "prefers GH_TOKEN", ghToken: " fine-grained ", githubToken: "other", want: "fine-grained"},
		{name: "falls back to GITHUB_TOKEN", ghToken: " ", githubToken: " fallback ", want: "fallback"},
		{name: "no token", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := tokenFromEnv(test.ghToken, test.githubToken); got != test.want {
				t.Fatalf("tokenFromEnv() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoadUsesUserLookupForPersonalProjectURL(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
		}
		receivedQuery = request.Query
		_, _ = w.Write([]byte(projectResponseJSON("user")))
	}))
	defer server.Close()

	ref, err := ParseProjectURL("https://github.com/users/DewyDozenal/projects/1")
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{http: server.Client(), token: "test-token", endpoint: server.URL}
	project, err := client.Load(ref)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if project.Title != "Test project" {
		t.Fatalf("project title = %q, want %q", project.Title, "Test project")
	}
	if !strings.Contains(receivedQuery, "user(login: $owner)") ||
		strings.Contains(receivedQuery, "organization(login: $owner)") {
		t.Fatalf("user project query uses wrong owner lookup: %s", receivedQuery)
	}
}

func TestLoadRetriesUserLookupForAmbiguousOwnerURL(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
		}
		queries = append(queries, request.Query)
		if strings.Contains(request.Query, "organization(login: $owner)") {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Could not resolve to an Organization with the login 'DewyDozenal'."}]}`))
			return
		}
		_, _ = w.Write([]byte(projectResponseJSON("user")))
	}))
	defer server.Close()

	ref, err := ParseProjectURL("https://github.com/DewyDozenal/projects/1")
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{http: server.Client(), token: "test-token", endpoint: server.URL}
	if _, err := client.Load(ref); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("received %d GraphQL queries, want organization lookup then user lookup", len(queries))
	}
	if !strings.Contains(queries[0], "organization(login: $owner)") ||
		!strings.Contains(queries[1], "user(login: $owner)") {
		t.Fatalf("owner lookup order was unexpected: %v", queries)
	}
}

func TestAddCommentPostsIssueComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query     string `json:"query"`
			Variables struct {
				ID   string `json:"id"`
				Body string `json:"body"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode GraphQL request: %v", err)
		}
		if !strings.Contains(request.Query, "addComment(input:") {
			t.Errorf("query does not add a comment: %s", request.Query)
		}
		if request.Variables.ID != "issue-id" || request.Variables.Body != "Looks good!" {
			t.Errorf("comment variables = %+v, want issue ID and comment body", request.Variables)
		}
		_, _ = w.Write([]byte(`{"data":{"addComment":{"commentEdge":{"node":{"id":"comment-id"}}}}}`))
	}))
	defer server.Close()

	client := &Client{http: server.Client(), token: "test-token", endpoint: server.URL}
	if err := client.AddComment(Issue{ID: "issue-id"}, "Looks good!"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
}

func projectResponseJSON(ownerType string) string {
	return `{"data":{"` + ownerType + `":{"projectV2":{"id":"project-id","title":"Test project","fields":{"nodes":[{"id":"status-id","name":"Status","options":[{"id":"todo-id","name":"Todo"}]}]},"items":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}}`
}
