package github

import "testing"

func TestParseProjectURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		owner     string
		number    int
		wantError bool
	}{
		{name: "organization project", url: "https://github.com/orgs/acme/projects/17", owner: "acme", number: 17},
		{name: "user project", url: "https://github.com/users/octocat/projects/2?tab=items", owner: "octocat", number: 2},
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
			if got.Owner != test.owner || got.Number != test.number {
				t.Fatalf("ParseProjectURL() = %+v, want owner %q and number %d", got, test.owner, test.number)
			}
		})
	}
}
