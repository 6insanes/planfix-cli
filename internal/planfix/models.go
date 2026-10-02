package planfix

// PersonRef is a user:N / contact:N / group:N reference.
type PersonRef struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
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
