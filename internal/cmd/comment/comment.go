// Package comment implements planfix comment list|add.
package comment

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/output"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

// Options are the global flags the comment commands honour.
type Options struct {
	JSON   bool
	Fields string
	Quiet  bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

// defaultListFields is requested when --fields is empty.
const defaultListFields = "id,description,dateTime,owner"

// NewCmd builds the comment command group (list and add).
func NewCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Work with Planfix task comments",
	}
	commentCmd.AddCommand(newListCmd(getClient, getOpts))
	commentCmd.AddCommand(newAddCmd(getClient, getOpts))
	return commentCmd
}

// parseTaskID validates the shared <task-id> positional argument.
func parseTaskID(args []string) (int, error) {
	id, err := strconv.Atoi(args[0])
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid task id %q", args[0])
	}
	return id, nil
}

func newListCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List comments on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseTaskID(args)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			opts := getOpts()
			fields := opts.Fields
			if fields == "" {
				fields = defaultListFields
			}
			list, raw, err := c.ListComments(cmd.Context(), id, fields)
			if err != nil {
				return planfix.WrapHint(err)
			}
			switch {
			case opts.JSON:
				return output.JSON(cmd.OutOrStdout(), raw)
			case opts.Quiet:
				rows := make([][]string, 0, len(list.Comments))
				for _, cm := range list.Comments {
					rows = append(rows, []string{strconv.Itoa(cm.ID)})
				}
				output.Table(cmd.OutOrStdout(), nil, rows)
			default:
				rows := make([][]string, 0, len(list.Comments))
				for _, cm := range list.Comments {
					rows = append(rows, []string{
						strconv.Itoa(cm.ID),
						cm.Timestamp.String(),
						cm.Author.Name,
						cm.Text,
					})
				}
				output.Table(cmd.OutOrStdout(), []string{"ID", "CREATED", "AUTHOR", "TEXT"}, rows)
			}
			return nil
		},
	}
}

func newAddCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var body string
	var silent bool

	cmd := &cobra.Command{
		Use:   "add <task-id>",
		Short: "Add a comment to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseTaskID(args)
			if err != nil {
				return err
			}
			text := body
			if text == "" {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read comment text: %w", err)
				}
				text = string(data)
			}
			if text == "" {
				return fmt.Errorf("--body or stdin text is required")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			cid, raw, err := c.AddComment(cmd.Context(), id, text, silent)
			if err != nil {
				return planfix.WrapHint(err)
			}
			opts := getOpts()
			return output.WriteResult(cmd.OutOrStdout(),
				output.WriteOpts{JSON: opts.JSON, Quiet: opts.Quiet},
				"Added", "comment", cid, raw)
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "comment text (reads stdin if empty)")
	cmd.Flags().BoolVar(&silent, "silent", false, "add without notifications")
	return cmd
}
