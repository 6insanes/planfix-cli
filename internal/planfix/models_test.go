package planfix

import (
	"encoding/json"
	"testing"
)

func TestPersonRefUnmarshalIDShapes(t *testing.T) {
	cases := []struct {
		name string
		json string
		want PersonRef
	}{
		{"number id", `{"id":5,"name":"Ann"}`, PersonRef{ID: 5, Name: "Ann"}},
		{"numeric string id", `{"id":"5","name":"Ann"}`, PersonRef{ID: 5, Name: "Ann"}},
		{"prefixed user id", `{"id":"user:5","name":"Ann"}`, PersonRef{ID: 5, Type: "user", Name: "Ann"}},
		{"prefixed contact id", `{"id":"contact:7","name":"Bob"}`, PersonRef{ID: 7, Type: "contact", Name: "Bob"}},
		{"prefixed group id", `{"id":"group:3","name":"Devs"}`, PersonRef{ID: 3, Type: "group", Name: "Devs"}},
		{"explicit type round-trip", `{"id":7,"type":"user","name":"Ann"}`, PersonRef{ID: 7, Type: "user", Name: "Ann"}},
		{"missing id", `{"name":"Ann"}`, PersonRef{Name: "Ann"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var got PersonRef
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal(%s) error = %v", tt.json, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %+v, want %+v", tt.json, got, tt.want)
			}
		})
	}
}

func TestPersonRefUnmarshalRejectsBadID(t *testing.T) {
	for _, in := range []string{`{"id":"user:x"}`, `{"id":"abc"}`, `{"id":true}`} {
		var p PersonRef
		if err := json.Unmarshal([]byte(in), &p); err == nil {
			t.Errorf("Unmarshal(%s) error = nil, want failure", in)
		}
	}
}

// TestUnmarshalTaskAssigneesStringIDs is the regression for task view
// failing on `json: cannot unmarshal string into ... id of type int`:
// the REST API returns assignee ids as prefixed strings.
func TestUnmarshalTaskAssigneesStringIDs(t *testing.T) {
	raw := `{"result":"success","task":{"id":2280555,"assignees":{"users":[
		{"id":"user:5","name":"Ann"},
		{"id":"contact:6","name":"Bob"}
	]}}}`
	var envelope struct {
		Task Task `json:"task"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	users := envelope.Task.Assignees.Users
	if len(users) != 2 {
		t.Fatalf("users = %+v, want 2 entries", users)
	}
	if users[0] != (PersonRef{ID: 5, Type: "user", Name: "Ann"}) {
		t.Errorf("users[0] = %+v", users[0])
	}
	if users[1] != (PersonRef{ID: 6, Type: "contact", Name: "Bob"}) {
		t.Errorf("users[1] = %+v", users[1])
	}
}

func TestTimePointString(t *testing.T) {
	cases := []struct {
		tp   TimePoint
		want string
	}{
		{TimePoint{Date: "01-06-2022", Time: "12:25", Datetime: "2022-06-01T12:25Z"}, "01-06-2022 12:25"},
		{TimePoint{Date: "01-06-2022"}, "01-06-2022"},
		{TimePoint{Datetime: "2022-06-01T12:25Z"}, "2022-06-01T12:25Z"},
	}
	for _, tt := range cases {
		if got := tt.tp.String(); got != tt.want {
			t.Errorf("(%+v).String() = %q, want %q", tt.tp, got, tt.want)
		}
	}
}

func TestCommentUnmarshalWireNames(t *testing.T) {
	raw := `{"id":10,"description":"hi there","type":"Comment",
		"dateTime":{"date":"01-01-2026","time":"10:00"},
		"owner":{"id":"user:3","name":"Ann"}}`
	var cm Comment
	if err := json.Unmarshal([]byte(raw), &cm); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cm.ID != 10 || cm.Text != "hi there" {
		t.Errorf("comment = %+v", cm)
	}
	if cm.Timestamp.String() != "01-01-2026 10:00" {
		t.Errorf("timestamp = %q", cm.Timestamp.String())
	}
	if cm.Author != (PersonRef{ID: 3, Type: "user", Name: "Ann"}) {
		t.Errorf("author = %+v", cm.Author)
	}
}
