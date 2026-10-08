package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const graphQLEndpoint = "https://api.github.com/graphql"

type ProjectRef struct {
	Owner     string
	OwnerType OwnerType
	Number    int
}

type OwnerType string

const (
	OrganizationOwner OwnerType = "organization"
	UserOwner         OwnerType = "user"
)

func ParseProjectURL(raw string) (ProjectRef, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() != "github.com" {
		return ProjectRef{}, errors.New("enter a GitHub project URL, such as https://github.com/orgs/OWNER/projects/1")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 4 && (parts[0] == "orgs" || parts[0] == "users") && parts[2] == "projects" {
		number, err := strconv.Atoi(parts[3])
		if err == nil && number > 0 {
			ownerType := OrganizationOwner
			if parts[0] == "users" {
				ownerType = UserOwner
			}
			return ProjectRef{Owner: parts[1], OwnerType: ownerType, Number: number}, nil
		}
	}
	if len(parts) == 3 && parts[1] == "projects" {
		number, err := strconv.Atoi(parts[2])
		if err == nil && number > 0 {
			return ProjectRef{Owner: parts[0], Number: number}, nil
		}
	}
	return ProjectRef{}, errors.New("project URL must point to a GitHub Projects v2 board")
}

type Client struct {
	http     *http.Client
	token    string
	endpoint string
}

func NewClient() (*Client, error) {
	token := tokenFromEnv(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		output, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, errors.New("GitHub authentication not found; set GH_TOKEN or GITHUB_TOKEN, or run `gh auth login`")
		}
		token = strings.TrimSpace(string(output))
		if token == "" {
			return nil, errors.New("GitHub returned an empty token; set GH_TOKEN or GITHUB_TOKEN, or run `gh auth login`")
		}
	}
	return &Client{
		http:     &http.Client{Timeout: 30 * time.Second},
		token:    token,
		endpoint: graphQLEndpoint,
	}, nil
}

func tokenFromEnv(ghToken, githubToken string) string {
	if token := strings.TrimSpace(ghToken); token != "" {
		return token
	}
	return strings.TrimSpace(githubToken)
}

type Project struct {
	ID            string
	StatusFieldID string
	Title         string
	Stages        []Stage
	Issues        []Issue
}

type Stage struct {
	ID   string
	Name string
}

type Issue struct {
	ItemID    string
	ID        string
	Title     string
	Body      string
	URL       string
	Number    int
	State     string
	Repo      string
	Labels    []string
	Assignees []string
	Stage     string
}

const projectFieldsFragment = `
fragment ProjectFields on ProjectV2 {
  id title
  fields(first: 100) {
    nodes {
      ... on ProjectV2SingleSelectField {
        id name options { id name }
      }
    }
  }
  items(first: 100, after: $cursor) {
    pageInfo { hasNextPage endCursor }
    nodes {
      id
      content {
        __typename
        ... on Issue {
          id title body url number state
          repository { nameWithOwner }
          labels(first: 30) { nodes { name } }
          assignees(first: 10) { nodes { login } }
        }
      }
      fieldValues(first: 50) {
        nodes {
          ... on ProjectV2ItemFieldSingleSelectValue {
            name optionId field { ... on ProjectV2SingleSelectField { id name } }
          }
        }
      }
    }
  }
}`

const organizationProjectQuery = `
query($owner: String!, $number: Int!, $cursor: String) {
  organization(login: $owner) {
    projectV2(number: $number) { ...ProjectFields }
  }
}` + projectFieldsFragment

const userProjectQuery = `
query($owner: String!, $number: Int!, $cursor: String) {
  user(login: $owner) {
    projectV2(number: $number) { ...ProjectFields }
  }
}` + projectFieldsFragment

type projectResponse struct {
	Organization *projectData `json:"organization"`
	User         *projectData `json:"user"`
}

type projectData struct {
	Project *struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Fields struct {
			Nodes []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Options []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"options"`
			} `json:"nodes"`
		} `json:"fields"`
		Items struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				ID      string `json:"id"`
				Content *struct {
					Type       string `json:"__typename"`
					ID         string `json:"id"`
					Title      string `json:"title"`
					Body       string `json:"body"`
					URL        string `json:"url"`
					Number     int    `json:"number"`
					State      string `json:"state"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
					Assignees struct {
						Nodes []struct {
							Login string `json:"login"`
						} `json:"nodes"`
					} `json:"assignees"`
				} `json:"content"`
				FieldValues struct {
					Nodes []struct {
						Name     string `json:"name"`
						OptionID string `json:"optionId"`
						Field    struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"field"`
					} `json:"nodes"`
				} `json:"fieldValues"`
			} `json:"nodes"`
		} `json:"items"`
	} `json:"projectV2"`
}

func (c *Client) Load(ref ProjectRef) (*Project, error) {
	ownerTypes := []OwnerType{ref.OwnerType}
	if ref.OwnerType == "" {
		ownerTypes = []OwnerType{OrganizationOwner, UserOwner}
	}
	var lastErr error
	for _, ownerType := range ownerTypes {
		project, err := c.loadProject(ref, ownerType)
		if err == nil {
			return project, nil
		}
		if lastErr == nil {
			lastErr = err
		}
	}
	return nil, lastErr
}

func (c *Client) loadProject(ref ProjectRef, ownerType OwnerType) (*Project, error) {
	var result *Project
	var cursor any
	for {
		var response projectResponse
		variables := map[string]any{"owner": ref.Owner, "number": ref.Number, "cursor": cursor}
		query := organizationProjectQuery
		if ownerType == UserOwner {
			query = userProjectQuery
		}
		if err := c.graphQL(query, variables, &response); err != nil {
			return nil, err
		}
		var data *projectData
		if ownerType == OrganizationOwner && response.Organization != nil {
			data = response.Organization
		} else if ownerType == UserOwner && response.User != nil {
			data = response.User
		}
		if data == nil || data.Project == nil {
			return nil, fmt.Errorf("project not found or inaccessible: check the URL and your GitHub permissions")
		}

		if result == nil {
			result = &Project{ID: data.Project.ID, Title: data.Project.Title}
			for _, field := range data.Project.Fields.Nodes {
				if strings.EqualFold(field.Name, "status") {
					result.StatusFieldID = field.ID
					for _, option := range field.Options {
						result.Stages = append(result.Stages, Stage{ID: option.ID, Name: option.Name})
					}
					break
				}
			}
			if result.StatusFieldID == "" {
				return nil, errors.New("this project has no single-select Status field")
			}
		}
		for _, item := range data.Project.Items.Nodes {
			if item.Content == nil || item.Content.Type != "Issue" {
				continue
			}
			issue := Issue{
				ItemID: item.ID, ID: item.Content.ID, Title: item.Content.Title,
				Body: item.Content.Body, URL: item.Content.URL, Number: item.Content.Number,
				State: item.Content.State, Repo: item.Content.Repository.NameWithOwner,
			}
			for _, label := range item.Content.Labels.Nodes {
				issue.Labels = append(issue.Labels, label.Name)
			}
			for _, assignee := range item.Content.Assignees.Nodes {
				issue.Assignees = append(issue.Assignees, assignee.Login)
			}
			for _, value := range item.FieldValues.Nodes {
				if value.Field.ID == result.StatusFieldID {
					issue.Stage = value.Name
					break
				}
			}
			if issue.Stage == "" {
				issue.Stage = "No status"
			}
			result.Issues = append(result.Issues, issue)
		}
		if !data.Project.Items.PageInfo.HasNextPage {
			break
		}
		if data.Project.Items.PageInfo.EndCursor == "" || data.Project.Items.PageInfo.EndCursor == cursor {
			return nil, errors.New("GitHub returned an invalid project pagination cursor")
		}
		cursor = data.Project.Items.PageInfo.EndCursor
	}
	if hasUnassignedIssue(result.Issues) {
		result.Stages = append(result.Stages, Stage{Name: "No status"})
	}
	return result, nil
}

func (c *Client) MoveIssue(projectID, fieldID string, issue Issue, stage Stage) error {
	if stage.ID == "" {
		const mutation = `mutation($project: ID!, $item: ID!, $field: ID!) {
			clearProjectV2ItemFieldValue(input: {
				projectId: $project, itemId: $item, fieldId: $field
			}) { projectV2Item { id } }
		}`
		return c.graphQL(mutation, map[string]any{
			"project": projectID, "item": issue.ItemID,
			"field": fieldID,
		}, nil)
	}
	const mutation = `mutation($project: ID!, $item: ID!, $field: ID!, $option: String!) {
		updateProjectV2ItemFieldValue(input: {
			projectId: $project, itemId: $item, fieldId: $field,
			value: { singleSelectOptionId: $option }
		}) { projectV2Item { id } }
	}`
	return c.graphQL(mutation, map[string]any{
		"project": projectID, "item": issue.ItemID, "field": fieldID, "option": stage.ID,
	}, nil)
}

func hasUnassignedIssue(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Stage == "No status" {
			return true
		}
	}
	return false
}

func (c *Client) EditIssue(issue Issue, title, body string) error {
	const mutation = `mutation($id: ID!, $title: String!, $body: String!) {
		updateIssue(input: {id: $id, title: $title, body: $body}) {
			issue { id }
		}
	}`
	return c.graphQL(mutation, map[string]any{
		"id": issue.ID, "title": title, "body": body,
	}, nil)
}

func (c *Client) graphQL(query string, variables map[string]any, target any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	endpoint := c.endpoint
	if endpoint == "" {
		endpoint = graphQLEndpoint
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("contacting GitHub: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("reading GitHub response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decoding GitHub response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, apiError := range envelope.Errors {
			messages = append(messages, apiError.Message)
		}
		return errors.New(strings.Join(messages, "; "))
	}
	if target != nil {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("decoding GitHub data: %w", err)
		}
	}
	return nil
}
