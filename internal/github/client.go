package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// GraphQLExecutor defines the contract for sending GraphQL requests.
type GraphQLExecutor interface {
	DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

// ProjectBoard represents a GitHub Projects v2 Kanban board.
type ProjectBoard struct {
	ID            string
	Title         string
	StatusFieldID string
	HoursFieldID  string
	Columns       []Column
	Items         []ProjectItem
}

// Column represents a single status column on the Kanban board.
type Column struct {
	ID    string
	Name  string
	Color string
}

// ProjectItem represents a single card on the board.
type ProjectItem struct {
	ID             string
	Type           string
	StatusOptionID string
	HoursSpent     float64
	Title          string
	Body           string
	Number         int
	State          string
	Labels         []Label
	Milestone      *Milestone
}

// Label represents an issue or PR label.
type Label struct {
	Name  string
	Color string
}

// Milestone represents an issue milestone badge.
type Milestone struct {
	Title string
	DueOn string
}

// Client wraps GraphQL communication for GitHub Projects v2.
type Client struct {
	gql GraphQLExecutor
}

// NewClient initializes a new GitHub Projects v2 client.
func NewClient(gql GraphQLExecutor) (*Client, error) {
	if gql == nil {
		client, err := api.DefaultGraphQLClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create default GitHub GraphQL client: %w", err)
		}
		gql = client
	}
	return &Client{gql: gql}, nil
}

// CheckProjectScope checks whether the user's gh CLI token has the 'project' scope.
func CheckProjectScope(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	if strings.Contains(outStr, "'project'") || strings.Contains(outStr, "'read:project'") {
		return nil
	}
	if strings.Contains(outStr, "Logged in to") && !strings.Contains(outStr, "project") {
		return errors.New("GitHub CLI lacks 'project' scope. Run 'gh auth refresh -s project' to grant permissions.")
	}
	return nil
}

const fetchProjectBoardQuery = `
query FetchProjectBoard($owner: String!, $projectNumber: Int!, $cursor: String) {
  repositoryOwner(login: $owner) {
    ... on User {
      projectV2(number: $projectNumber) {
        ...ProjectFields
      }
    }
    ... on Organization {
      projectV2(number: $projectNumber) {
        ...ProjectFields
      }
    }
  }
}

fragment ProjectFields on ProjectV2 {
  id
  title
  fields(first: 50) {
    nodes {
      ... on ProjectV2SingleSelectField {
        id
        name
        options {
          id
          name
          color
        }
      }
      ... on ProjectV2Field {
        id
        name
        dataType
      }
    }
  }
  items(first: 100, after: $cursor) {
    pageInfo {
      hasNextPage
      endCursor
    }
    nodes {
      id
      type
      fieldValues(first: 10) {
        nodes {
          ... on ProjectV2ItemFieldSingleSelectValue {
            name
            optionId
            field {
              ... on ProjectV2SingleSelectField {
                id
                name
              }
            }
          }
          ... on ProjectV2ItemFieldNumberValue {
            number
            field {
              ... on ProjectV2Field {
                id
                name
              }
            }
          }
        }
      }
      content {
        ... on Issue {
          id
          number
          title
          body
          state
          milestone {
            title
            dueOn
          }
          labels(first: 10) {
            nodes {
              name
              color
            }
          }
        }
        ... on PullRequest {
          id
          number
          title
          state
          labels(first: 10) {
            nodes {
              name
              color
            }
          }
        }
        ... on DraftIssue {
          id
          title
          body
        }
      }
    }
  }
}
`

// Raw GraphQL response structures
type gqlResponse struct {
	RepositoryOwner struct {
		ProjectV2 *gqlProject `json:"projectV2"`
	} `json:"repositoryOwner"`
}

type gqlProject struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Fields struct {
		Nodes []gqlField `json:"nodes"`
	} `json:"fields"`
	Items struct {
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes []gqlItem `json:"nodes"`
	} `json:"items"`
}

type gqlField struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	DataType string           `json:"dataType"`
	Options  []gqlFieldOption `json:"options,omitempty"`
}

type gqlFieldOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type gqlItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	FieldValues struct {
		Nodes []gqlFieldValue `json:"nodes"`
	} `json:"fieldValues"`
	Content *gqlContent `json:"content,omitempty"`
}

type gqlFieldValue struct {
	Name     string   `json:"name"`
	OptionID string   `json:"optionId"`
	Number   *float64 `json:"number"`
	Field    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"field"`
}

type gqlContent struct {
	ID        string `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Milestone *struct {
		Title string `json:"title"`
		DueOn string `json:"dueOn"`
	} `json:"milestone"`
	Labels struct {
		Nodes []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"nodes"`
	} `json:"labels"`
}

// FetchBoard fetches the project board metadata, status columns, custom fields, and items.
func (c *Client) FetchBoard(ctx context.Context, owner string, number int, hoursFieldName string) (*ProjectBoard, error) {
	if hoursFieldName == "" {
		hoursFieldName = "Hours Spent"
	}

	board := &ProjectBoard{}
	var cursor *string
	hasMore := true

	for hasMore {
		var vars = map[string]interface{}{
			"owner":         owner,
			"projectNumber": number,
			"cursor":        cursor,
		}

		var resp gqlResponse
		if err := c.gql.DoWithContext(ctx, fetchProjectBoardQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("GraphQL query failed: %w", err)
		}

		proj := resp.RepositoryOwner.ProjectV2
		if proj == nil {
			return nil, fmt.Errorf("project #%d not found for owner '%s'", number, owner)
		}

		if board.ID == "" {
			board.ID = proj.ID
			board.Title = proj.Title

			var statusField *gqlField
			var fallbackSelectField *gqlField

			// Inspect fields
			for i := range proj.Fields.Nodes {
				f := &proj.Fields.Nodes[i]
				if len(f.Options) > 0 {
					if strings.EqualFold(f.Name, "status") {
						statusField = f
					} else if fallbackSelectField == nil {
						fallbackSelectField = f
					}
				}
				if strings.EqualFold(f.Name, hoursFieldName) {
					board.HoursFieldID = f.ID
				}
			}

			// If no field explicitly named "status" (case-insensitive) was found,
			// fall back to the first available single-select field with options.
			if statusField == nil {
				statusField = fallbackSelectField
			}

			if statusField != nil {
				board.StatusFieldID = statusField.ID
				for _, opt := range statusField.Options {
					board.Columns = append(board.Columns, Column{
						ID:    opt.ID,
						Name:  opt.Name,
						Color: opt.Color,
					})
				}
			}
		}

		for _, itemNode := range proj.Items.Nodes {
			item := ProjectItem{
				ID:   itemNode.ID,
				Type: itemNode.Type,
			}

			// Parse field values
			for _, fv := range itemNode.FieldValues.Nodes {
				if fv.Field.ID == board.StatusFieldID || (board.StatusFieldID == "" && strings.EqualFold(fv.Field.Name, "status")) {
					item.StatusOptionID = fv.OptionID
				}
				if fv.Field.ID == board.HoursFieldID || strings.EqualFold(fv.Field.Name, hoursFieldName) {
					if fv.Number != nil {
						item.HoursSpent = *fv.Number
					}
				}
			}

			// Parse content
			if itemNode.Content != nil {
				item.Number = itemNode.Content.Number
				item.Title = itemNode.Content.Title
				item.Body = itemNode.Content.Body
				item.State = itemNode.Content.State

				if itemNode.Content.Milestone != nil {
					item.Milestone = &Milestone{
						Title: itemNode.Content.Milestone.Title,
						DueOn: itemNode.Content.Milestone.DueOn,
					}
				}

				for _, l := range itemNode.Content.Labels.Nodes {
					item.Labels = append(item.Labels, Label{
						Name:  l.Name,
						Color: l.Color,
					})
				}
			}

			board.Items = append(board.Items, item)
		}

		hasMore = proj.Items.PageInfo.HasNextPage
		if hasMore {
			cStr := proj.Items.PageInfo.EndCursor
			cursor = &cStr
		}
	}

	return board, nil
}

const moveCardMutation = `
mutation MoveCardStatus($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) {
  updateProjectV2ItemFieldValue(
    input: {
      projectId: $projectId
      itemId: $itemId
      fieldId: $fieldId
      value: { singleSelectOptionId: $optionId }
    }
  ) {
    projectV2Item { id }
  }
}
`

// MoveCard moves a card to a different status column on the Kanban board.
func (c *Client) MoveCard(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	vars := map[string]interface{}{
		"projectId": projectID,
		"itemId":    itemID,
		"fieldId":   fieldID,
		"optionId":  optionID,
	}
	var resp struct{}
	if err := c.gql.DoWithContext(ctx, moveCardMutation, vars, &resp); err != nil {
		return fmt.Errorf("failed to move card status: %w", err)
	}
	return nil
}

const updateHoursMutation = `
mutation UpdateHoursSpent($projectId: ID!, $itemId: ID!, $fieldId: ID!, $totalHours: Float!) {
  updateProjectV2ItemFieldValue(
    input: {
      projectId: $projectId
      itemId: $itemId
      fieldId: $fieldId
      value: { number: $totalHours }
    }
  ) {
    projectV2Item { id }
  }
}
`

// UpdateHours updates the custom float field on a project item with new total hours.
func (c *Client) UpdateHours(ctx context.Context, projectID, itemID, fieldID string, totalHours float64) error {
	vars := map[string]interface{}{
		"projectId":  projectID,
		"itemId":     itemID,
		"fieldId":    fieldID,
		"totalHours": totalHours,
	}
	var resp struct{}
	if err := c.gql.DoWithContext(ctx, updateHoursMutation, vars, &resp); err != nil {
		return fmt.Errorf("failed to update hours spent: %w", err)
	}
	return nil
}
