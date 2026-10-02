// Package user implements planfix user read commands.
package user

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"planfix-cli/internal/output"
	"planfix-cli/internal/planfix"
)

// Options are the global flags the user commands honour.
type Options struct {
	JSON   bool
	Fields string
	Quiet  bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

const (
	// defaultListFields is requested when --fields is empty.
	defaultListFields = "id,name,email"
	// defaultListLimit is both the --limit flag default and the fallback
	// when --limit is passed a non-positive value.
	defaultListLimit = 50
)

// NewCmd builds the user command group.
func NewCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Work with Planfix users",
	}
	userCmd.AddCommand(newListCmd(getClient, getOpts))
	return userCmd
}

func newListCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			opts := getOpts()
			fields := opts.Fields
			if fields == "" {
				fields = defaultListFields
			}
			if limit <= 0 {
				limit = defaultListLimit
			}
			list, raw, err := c.ListUsers(cmd.Context(), offset, limit, fields)
			if err != nil {
				return planfix.WrapHint(err)
			}
			switch {
			case opts.JSON:
				return output.JSON(cmd.OutOrStdout(), raw)
			case opts.Quiet:
				rows := make([][]string, 0, len(list.Users))
				for _, u := range list.Users {
					rows = append(rows, []string{strconv.Itoa(u.ID)})
				}
				output.Table(cmd.OutOrStdout(), nil, rows)
			default:
				names, headers := fieldColumns(fields)
				rows := make([][]string, 0, len(list.Users))
				for _, u := range list.Users {
					row := make([]string, 0, len(names))
					for _, name := range names {
						row = append(row, fieldValue(u, name))
					}
					rows = append(rows, row)
				}
				output.Table(cmd.OutOrStdout(), headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "page size")
	cmd.Flags().IntVar(&offset, "offset", 0, "offset")
	return cmd
}

// fieldColumns splits a comma-separated --fields value into trimmed,
// non-empty API field names and their display headers ("id" -> "ID"),
// preserving order. Empty input yields nil slices.
func fieldColumns(fields string) (names, headers []string) {
	for _, p := range strings.Split(fields, ",") {
		f := strings.TrimSpace(p)
		if f == "" {
			continue
		}
		names = append(names, f)
		headers = append(headers, strings.ToUpper(f))
	}
	return names, headers
}

// fieldValue renders one user field as its display string. Unknown fields
// render empty so output stays aligned with the requested list.
func fieldValue(u planfix.User, field string) string {
	switch field {
	case "id":
		return strconv.Itoa(u.ID)
	case "name":
		return u.Name
	case "email":
		return u.Email
	case "status":
		return u.Status
	}
	return ""
}
