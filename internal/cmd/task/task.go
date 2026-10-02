// Package task implements planfix task read commands.
package task

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"planfix-cli/internal/output"
	"planfix-cli/internal/planfix"
)

// Options are the global flags the task commands honour.
type Options struct {
	JSON   bool
	Fields string
	Quiet  bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

// defaultListLimit is both the --limit flag default and the fallback when
// --limit is passed a non-positive value.
const defaultListLimit = 50

// NewCmd builds the task command group (read and write commands).
// getDomain resolves the active profile's domain for `task open`.
func NewCmd(getClient ClientFunc, getOpts func() Options, getDomain func() string) *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Work with Planfix tasks",
	}
	taskCmd.AddCommand(newListCmd(getClient, getOpts))
	taskCmd.AddCommand(newViewCmd(getClient, getOpts))
	NewWriteCmd(taskCmd, getClient, getOpts, getDomain)
	return taskCmd
}

func newListCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var limit, offset int
	var filter, savedFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
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
			list, raw, err := c.ListTasks(cmd.Context(), planfix.ListTasksRequest{
				Offset:      offset,
				PageSize:    limit,
				Fields:      fields,
				FilterJSON:  filter,
				SavedFilter: savedFilter,
			})
			if err != nil {
				return planfix.WrapHint(err)
			}
			if opts.JSON {
				return output.JSON(cmd.OutOrStdout(), raw)
			}
			if opts.Quiet {
				rows := make([][]string, 0, len(list.Tasks))
				for _, t := range list.Tasks {
					rows = append(rows, []string{strconv.Itoa(t.ID)})
				}
				output.Table(cmd.OutOrStdout(), nil, rows)
				return nil
			}
			names, headers := fieldColumns(fields)
			rows := make([][]string, 0, len(list.Tasks))
			for _, t := range list.Tasks {
				row := make([]string, 0, len(names))
				for _, name := range names {
					row = append(row, fieldValue(t, name))
				}
				rows = append(rows, row)
			}
			output.Table(cmd.OutOrStdout(), headers, rows)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "page size")
	cmd.Flags().IntVar(&offset, "offset", 0, "offset")
	cmd.Flags().StringVar(&filter, "filter", "", "raw Planfix filters JSON array")
	cmd.Flags().StringVar(&savedFilter, "saved-filter", "", "saved filter id (e.g. :in)")
	return cmd
}

func newViewCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "View one task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid task id %q", args[0])
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			opts := getOpts()
			fields := opts.Fields
			requestFields := fields
			if requestFields == "" {
				requestFields = defaultViewFields
			}
			t, raw, err := c.GetTask(cmd.Context(), id, requestFields)
			if err != nil {
				return planfix.WrapHint(err)
			}
			if opts.JSON {
				return output.JSON(cmd.OutOrStdout(), raw)
			}
			if opts.Quiet {
				fmt.Fprintln(cmd.OutOrStdout(), t.ID)
				return nil
			}
			var kv [][2]string
			if fields == "" {
				// Default rendering keeps a curated key order and skips empty
				// optional values instead of echoing the raw field list.
				kv = defaultViewKV(*t)
			} else {
				names, headers := fieldColumns(fields)
				kv = make([][2]string, 0, len(names))
				for i, name := range names {
					kv = append(kv, [2]string{headers[i], fieldValue(*t, name)})
				}
			}
			output.Detail(cmd.OutOrStdout(), kv)
			return nil
		},
	}
}
