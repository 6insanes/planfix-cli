package task

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/output"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

// NewWriteCmd adds the create/update/open commands to the task group.
func NewWriteCmd(taskCmd *cobra.Command, getClient ClientFunc, getOpts func() Options, getDomain func() string) {
	taskCmd.AddCommand(newCreateCmd(getClient, getOpts))
	taskCmd.AddCommand(newUpdateCmd(getClient, getOpts))
	taskCmd.AddCommand(newTakeCmd(getClient, getOpts))
	taskCmd.AddCommand(newOpenCmd(getDomain))
}

// parsePeople turns a comma-separated "user:N,contact:N,group:N" spec into
// PersonRefs. An empty spec yields nil.
func parsePeople(spec string) ([]planfix.PersonRef, error) {
	if spec == "" {
		return nil, nil
	}
	var out []planfix.PersonRef
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		typ, idStr, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("invalid person %q, want user:N|contact:N|group:N", part)
		}
		if typ != "user" && typ != "contact" && typ != "group" {
			return nil, fmt.Errorf("invalid person %q, want user:N|contact:N|group:N", part)
		}
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid person id in %q", part)
		}
		out = append(out, planfix.PersonRef{Type: typ, ID: id})
	}
	return out, nil
}

// writeTaskResult renders a create/update outcome via output.WriteResult.
func writeTaskResult(w io.Writer, opts Options, verb string, id int, raw []byte) error {
	return output.WriteResult(w, output.WriteOpts{JSON: opts.JSON, Quiet: opts.Quiet}, verb, "task", id, raw)
}

func newCreateCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var name, description, assignees, startDate, endDate string
	var project, parent int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			people, err := parsePeople(assignees)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			id, raw, err := c.CreateTask(cmd.Context(), planfix.CreateTaskRequest{
				Name:        name,
				Description: description,
				ProjectID:   project,
				ParentID:    parent,
				Assignees:   people,
				StartDate:   startDate,
				EndDate:     endDate,
			})
			if err != nil {
				return planfix.WrapHint(err)
			}
			return writeTaskResult(cmd.OutOrStdout(), getOpts(), "Created", id, raw)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "task name (required)")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().IntVar(&project, "project", 0, "project id")
	cmd.Flags().IntVar(&parent, "parent", 0, "parent task id")
	cmd.Flags().StringVar(&assignees, "assignees", "", "comma-separated user:N,contact:N,group:N")
	cmd.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	return cmd
}

func newUpdateCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var name, description, assignees, startDate, endDate string
	var status int

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid task id %q", args[0])
			}
			req := planfix.UpdateTaskRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("start-date") {
				req.StartDate = &startDate
			}
			if cmd.Flags().Changed("end-date") {
				req.EndDate = &endDate
			}
			if cmd.Flags().Changed("status") {
				if status <= 0 {
					return fmt.Errorf("--status must be a positive id")
				}
				req.Status = &status
			}
			if assignees != "" {
				people, err := parsePeople(assignees)
				if err != nil {
					return err
				}
				req.Assignees = people
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			raw, err := c.UpdateTask(cmd.Context(), id, req)
			if err != nil {
				return planfix.WrapHint(err)
			}
			return writeTaskResult(cmd.OutOrStdout(), getOpts(), "Updated", id, raw)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().IntVar(&status, "status", 0, "status id (see `task statuses`)")
	cmd.Flags().StringVar(&assignees, "assignees", "", "replace assignees (user:N,...)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	return cmd
}

func newOpenCmd(getDomain func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "open <id>",
		Short: "Print the task URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid task id %q", args[0])
			}
			domain := getDomain()
			if domain == "" {
				return fmt.Errorf("no domain configured; run `planfix auth login`")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "https://%s/task/%d\n", planfix.NormalizeDomain(domain), id)
			return nil
		},
	}
}
