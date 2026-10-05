package planfix

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PersonRef is a user:N / contact:N / group:N reference.
type PersonRef struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// UnmarshalJSON accepts every id shape the REST API returns: a JSON
// number, a numeric string, or a prefixed string ("user:5") whose prefix
// carries the type. The CLI's own {id,type} round-trips as well.
func (p *PersonRef) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID   any    `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	id, prefix, err := parsePersonID(raw.ID)
	if err != nil {
		return err
	}
	p.ID, p.Name, p.Type = id, raw.Name, raw.Type
	if prefix != "" {
		p.Type = prefix
	}
	return nil
}

// parsePersonID splits a wire person id into its number and type prefix:
// 5, "5", or "user:5" / "contact:5" / "group:5".
func parsePersonID(v any) (int, string, error) {
	switch x := v.(type) {
	case nil:
		return 0, "", nil
	case float64:
		return int(x), "", nil
	case string:
		num := x
		prefix := ""
		if p, n, ok := strings.Cut(x, ":"); ok {
			prefix, num = p, n
		}
		id, err := strconv.Atoi(num)
		if err != nil {
			return 0, "", fmt.Errorf("person id %q: %w", x, err)
		}
		return id, prefix, nil
	default:
		return 0, "", fmt.Errorf("person id: unexpected type %T", v)
	}
}

// MarshalJSON writes the wire form the REST API accepts: a prefixed
// string id ("user:5") for user/contact refs, a plain number for groups
// and untyped refs. The type prefix is the only type carrier, so the
// "type" key is not sent.
func (p PersonRef) MarshalJSON() ([]byte, error) {
	if p.Type != "" && p.Type != "group" {
		return json.Marshal(struct {
			ID string `json:"id"`
		}{ID: p.Type + ":" + strconv.Itoa(p.ID)})
	}
	return json.Marshal(struct {
		ID int `json:"id"`
	}{ID: p.ID})
}

// Status is a task/process status.
type Status struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

// ProjectRef is a nested project pointer on a task.
type ProjectRef struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

// TaskRef is a nested task pointer on a comment.
type TaskRef struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

// Task is a Planfix task (subset of fields the CLI uses).
type Task struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Status      Status      `json:"status"`
	Priority    string      `json:"priority,omitempty"`
	Project     *ProjectRef `json:"project,omitempty"`
	StartDate   string      `json:"startDate,omitempty"`
	EndDate     string      `json:"endDate,omitempty"`
	Assignees   struct {
		Users []PersonRef `json:"users,omitempty"`
	} `json:"assignees,omitempty"`
}

// TaskList is the POST /task/list envelope payload.
type TaskList struct {
	Tasks []Task `json:"tasks"`
}

// ListTasksRequest is the input to POST /task/list. Fields carry no JSON
// tags: ListTasks builds the wire shape in its method body (FilterJSON is
// validated then injected under "filters").
type ListTasksRequest struct {
	Offset      int
	PageSize    int
	Fields      string
	FilterJSON  string
	SavedFilter string
}
