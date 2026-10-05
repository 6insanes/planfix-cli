package task

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/output"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

// statusesTaskFields are the task fields fetched to link a task to its
// process and spot the current status.
const statusesTaskFields = "id,status,processId"

func newStatusesCmd(getClient ClientFunc, getOpts func() Options) *cobra.Command {
	return &cobra.Command{
		Use:   "statuses <id>",
		Short: "List statuses available for a task",
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
			t, _, err := c.GetTask(cmd.Context(), id, statusesTaskFields)
			if err != nil {
				return planfix.WrapHint(err)
			}
			list, raw, err := c.ObjectStatuses(cmd.Context(), id)
			if err != nil {
				objErr := err
				if t.ProcessID <= 0 {
					return planfix.WrapHint(objErr)
				}
				list, raw, err = c.ListTaskStatuses(cmd.Context(), t.ProcessID)
				if err != nil {
					return planfix.WrapHint(fmt.Errorf(
						"task statuses: /object/%d/statuses: %v; /process/task/%d/statuses: %w",
						id, objErr, t.ProcessID, err))
				}
			}
			switch {
			case opts.JSON:
				return output.JSON(cmd.OutOrStdout(), raw)
			case opts.Quiet:
				rows := make([][]string, 0, len(list.Statuses))
				for _, s := range list.Statuses {
					rows = append(rows, []string{strconv.Itoa(s.ID)})
				}
				output.Table(cmd.OutOrStdout(), nil, rows)
			default:
				rows := make([][]string, 0, len(list.Statuses))
				for _, s := range list.Statuses {
					current := ""
					if s.ID == t.Status.ID {
						current = "*"
					}
					rows = append(rows, []string{
						strconv.Itoa(s.ID),
						s.Name,
						strconv.FormatBool(s.IsActive),
						current,
					})
				}
				output.Table(cmd.OutOrStdout(), []string{"ID", "NAME", "ACTIVE", "CURRENT"}, rows)
			}
			return nil
		},
	}
}
