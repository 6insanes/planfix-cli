package planfix

import (
	"context"
	"encoding/json"
	"net/http"
)

// UserInfo is the GET /userinfo profile of the authenticated login.
type UserInfo struct {
	ID    PersonRef
	Name  string
	Login string
	Email string
	Type  string
}

// UnmarshalJSON reads the profile shape where the person id is a prefixed
// string ("user:123") rather than a nested object.
func (u *UserInfo) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID    any    `json:"id"`
		Name  string `json:"name"`
		Login string `json:"login"`
		Email string `json:"email"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	id, prefix, err := parsePersonID(raw.ID)
	if err != nil {
		return err
	}
	if prefix == "" {
		prefix = raw.Type
	}
	u.ID = PersonRef{ID: id, Type: prefix}
	u.Name, u.Login, u.Email, u.Type = raw.Name, raw.Login, raw.Email, raw.Type
	return nil
}

// GetUserInfo fetches the authenticated user's profile.
func (c *Client) GetUserInfo(ctx context.Context) (*UserInfo, error) {
	raw, err := c.JSON(ctx, http.MethodGet, "/userinfo", nil)
	if err != nil {
		return nil, err
	}
	var info UserInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
