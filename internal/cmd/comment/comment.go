// Package comment implements planfix comment list|add|edit|delete.
package comment

import (
	"bytes"
	"context"
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
	Plain  bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

// defaultListFields is requested when --fields is empty.
const defaultListFields = "id,description,dateTime,owner"

// NewCmd builds the comment command group (list, add, edit and delete).
func NewCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Work with Planfix task comments",
	}
	commentCmd.AddCommand(newListCmd(getClient, getOpts))
	commentCmd.AddCommand(newAddCmd(getClient, getOpts))
	commentCmd.AddCommand(newEditCmd(getClient, getOpts))
	commentCmd.AddCommand(newDeleteCmd(getClient, getOpts))
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

// parseIDs validates the <task-id> <comment-id> positional arguments.
func parseIDs(args []string) (int, int, error) {
	taskID, err := parseTaskID(args)
	if err != nil {
		return 0, 0, err
	}
	commentID, err := strconv.Atoi(args[1])
	if err != nil || commentID <= 0 {
		return 0, 0, fmt.Errorf("invalid comment id %q", args[1])
	}
	return taskID, commentID, nil
}

// readText resolves comment text from --body, falling back to stdin.
func readText(cmd *cobra.Command, body string) (string, error) {
	text := body
	if text == "" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read comment text: %w", err)
		}
		text = string(data)
	}
	if text == "" {
		return "", fmt.Errorf("--body or stdin text is required")
	}
	return text, nil
}

// requireCommentInTask verifies the comment lives on the task before a
// mutation: the delete endpoint takes no task id of its own.
func requireCommentInTask(ctx context.Context, c *planfix.Client, taskID, commentID int) error {
	cm, _, err := c.GetComment(ctx, commentID, "id,task")
	if err != nil {
		return err
	}
	if cm.Task == nil || cm.Task.ID != taskID {
		return fmt.Errorf("comment %d is not a comment on task %d", commentID, taskID)
	}
	return nil
}

// resultRaw falls back to a minimal id envelope for void API responses.
func resultRaw(raw []byte, id int) []byte {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte(fmt.Sprintf(`{"id":%d}`, id))
	}
	return raw
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
				output.Table(cmd.OutOrStdout(), []string{"ID", "CREATED", "AUTHOR", "TEXT"}, output.StripRows(rows, opts.Plain))
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
			text, err := readText(cmd, body)
			if err != nil {
				return err
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

func newEditCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var body string
	var silent bool

	cmd := &cobra.Command{
		Use:   "edit <task-id> <comment-id>",
		Short: "Edit a comment on a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, cid, err := parseIDs(args)
			if err != nil {
				return err
			}
			text, err := readText(cmd, body)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			if err := requireCommentInTask(cmd.Context(), c, id, cid); err != nil {
				return planfix.WrapHint(err)
			}
			raw, err := c.UpdateComment(cmd.Context(), id, cid, text, silent)
			if err != nil {
				return planfix.WrapHint(err)
			}
			opts := getOpts()
			return output.WriteResult(cmd.OutOrStdout(),
				output.WriteOpts{JSON: opts.JSON, Quiet: opts.Quiet},
				"Updated", "comment", cid, resultRaw(raw, cid))
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "new comment text (reads stdin if empty)")
	cmd.Flags().BoolVar(&silent, "silent", false, "edit without notifications")
	return cmd
}

func newDeleteCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var silent bool

	cmd := &cobra.Command{
		Use:   "delete <task-id> <comment-id>",
		Short: "Delete a comment from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, cid, err := parseIDs(args)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			if err := requireCommentInTask(cmd.Context(), c, id, cid); err != nil {
				return planfix.WrapHint(err)
			}
			raw, err := c.DeleteComment(cmd.Context(), cid, silent)
			if err != nil {
				return planfix.WrapHint(err)
			}
			opts := getOpts()
			return output.WriteResult(cmd.OutOrStdout(),
				output.WriteOpts{JSON: opts.JSON, Quiet: opts.Quiet},
				"Deleted", "comment", cid, resultRaw(raw, cid))
		},
	}

	cmd.Flags().BoolVar(&silent, "silent", false, "delete without notifications")
	return cmd
}
