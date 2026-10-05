package task

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/planfix"
)

// defaultTakeStatus is "В работе" / "In process" in the standard Planfix
// status set; --status overrides it.
const defaultTakeStatus = 2

// takeTaskFields are the task fields fetched before taking it.
const takeTaskFields = "id,assignees"

func newTakeCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	var status int

	cmd := &cobra.Command{
		Use:   "take <id>",
		Short: "Take a task into work (assign me and set status)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid task id %q", args[0])
			}
			if status <= 0 {
				return fmt.Errorf("--status must be a positive id")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			me, err := c.GetUserInfo(cmd.Context())
			if err != nil {
				return planfix.WrapHint(err)
			}
			t, _, err := c.GetTask(cmd.Context(), id, takeTaskFields)
			if err != nil {
				return planfix.WrapHint(err)
			}
			raw, err := c.UpdateTask(cmd.Context(), id, planfix.UpdateTaskRequest{
				Status:    &status,
				Assignees: withAssignee(*t, me.ID),
			})
			if err != nil {
				return planfix.WrapHint(err)
			}
			return writeTaskResult(cmd.OutOrStdout(), getOpts(), "Took", id, raw)
		},
	}

	cmd.Flags().IntVar(&status, "status", defaultTakeStatus, "status id to set (default: В работе)")
	return cmd
}

// withAssignee appends ref to the task's assignees unless already there.
// Response groups carry no type prefix, so they are tagged "group" to
// round-trip through the update payload.
func withAssignee(t planfix.Task, ref planfix.PersonRef) []planfix.PersonRef {
	normalize := func(p planfix.PersonRef, fallback string) planfix.PersonRef {
		if p.Type == "" {
			p.Type = fallback
		}
		return p
	}
	ref = normalize(ref, "user")
	out := make([]planfix.PersonRef, 0, len(t.Assignees.Users)+len(t.Assignees.Groups)+1)
	present := false
	for _, u := range t.Assignees.Users {
		u = normalize(u, "user")
		if u.Type == ref.Type && u.ID == ref.ID {
			present = true
		}
		out = append(out, u)
	}
	for _, g := range t.Assignees.Groups {
		g = normalize(g, "group")
		if g.Type == ref.Type && g.ID == ref.ID {
			present = true
		}
		out = append(out, g)
	}
	if !present {
		out = append(out, ref)
	}
	return out
}
