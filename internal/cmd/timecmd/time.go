// Package timecmd implements planfix time add|list (worklog).
package timecmd

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"planfix-cli/internal/config"
	"planfix-cli/internal/output"
	"planfix-cli/internal/planfix"
	"planfix-cli/internal/worklogtag"
)

// Options are the global flags the time commands honour.
type Options struct {
	JSON  bool
	Quiet bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

// MetaFunc resolves the account's worklog metadata, forcing re-discovery
// when refresh is set.
type MetaFunc func(ctx context.Context, c *planfix.Client, refresh bool) (*config.WorklogMeta, error)

// dateTimeLayout is the --from/--to argument format.
const dateTimeLayout = "2006-01-02 15:04"

// dateLayout is the --date argument format.
const dateLayout = "2006-01-02"

// apiDateLayout is the worklog date format Planfix expects (dd-MM-yyyy).
const apiDateLayout = "02-01-2006"

// timeLayout is the worklog clock-time format (HH:MM).
const timeLayout = "15:04"

// ParseHours parses a positive worked-hours value.
func ParseHours(s string) (float64, error) {
	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h <= 0 {
		return 0, fmt.Errorf("invalid hours %q: must be a positive number", s)
	}
	return h, nil
}

// ParseFromTo parses the --from/--to bounds and requires to > from.
func ParseFromTo(from, to string) (time.Time, time.Time, error) {
	f, err := time.Parse(dateTimeLayout, from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --from %q: want %q", from, dateTimeLayout)
	}
	t, err := time.Parse(dateTimeLayout, to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --to %q: want %q", to, dateTimeLayout)
	}
	if !t.After(f) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to %q must be after --from %q", to, from)
	}
	return f, t, nil
}

// NewCmd builds the time command group (add and list).
func NewCmd(getClient ClientFunc, getOpts func() Options, getMeta MetaFunc) *cobra.Command {
	timeCmd := &cobra.Command{
		Use:   "time",
		Short: "Track work time on tasks (worklog)",
	}
	timeCmd.AddCommand(newAddCmd(getClient, getOpts, getMeta))
	timeCmd.AddCommand(newListCmd(getClient, getOpts, getMeta))
	return timeCmd
}

// parseTaskID validates the shared <task-id> positional argument.
func parseTaskID(args []string) (int, error) {
	id, err := strconv.Atoi(args[0])
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid task id %q", args[0])
	}
	return id, nil
}

// buildEntry turns the interval flags into a WorklogEntry. Exactly one of
// --hours or --from/--to is required; --date overrides the calendar date.
func buildEntry(hours, from, to, dateFlag string, workType int) (planfix.WorklogEntry, error) {
	var e planfix.WorklogEntry
	e.WorkTypeKey = workType

	// parseDate is only called when dateFlag is set.
	parseDate := func() (time.Time, error) {
		d, err := time.Parse(dateLayout, dateFlag)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --date %q: want %q", dateFlag, dateLayout)
		}
		return d, nil
	}

	switch {
	case hours != "" && (from != "" || to != ""):
		return e, fmt.Errorf("--hours and --from/--to are mutually exclusive")
	case hours != "":
		h, err := ParseHours(hours)
		if err != nil {
			return e, err
		}
		end := time.Now()
		start := end.Add(-time.Duration(h * float64(time.Hour)))
		e.From = start.Format(timeLayout)
		e.To = end.Format(timeLayout)
		day := end
		if dateFlag != "" {
			if day, err = parseDate(); err != nil {
				return e, err
			}
		}
		e.Date = day.Format(apiDateLayout)
	case from != "" || to != "":
		if from == "" || to == "" {
			return e, fmt.Errorf("--from and --to must be used together")
		}
		f, t2, err := ParseFromTo(from, to)
		if err != nil {
			return e, err
		}
		day := f
		if dateFlag != "" {
			if day, err = parseDate(); err != nil {
				return e, err
			}
		}
		e.Date = day.Format(apiDateLayout)
		e.From = f.Format(timeLayout)
		e.To = t2.Format(timeLayout)
	default:
		return e, fmt.Errorf("--hours or --from/--to is required")
	}
	return e, nil
}

func newAddCmd(getClient ClientFunc, getOpts func() Options, getMeta MetaFunc) *cobra.Command {
	var hours, from, to, dateFlag, note string
	var workType int
	var refresh bool

	cmd := &cobra.Command{
		Use:   "add <task-id>",
		Short: "Log work time on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseTaskID(args)
			if err != nil {
				return err
			}
			entry, err := buildEntry(hours, from, to, dateFlag, workType)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			meta, err := getMeta(cmd.Context(), c, refresh)
			if err != nil {
				return planfix.WrapHint(err)
			}
			wid, raw, err := c.CreateWorklogEntry(cmd.Context(), id, meta, entry)
			if err != nil {
				return planfix.WrapHint(err)
			}
			if note != "" {
				if _, _, err := c.AddComment(cmd.Context(), id, note, true); err != nil {
					return planfix.WrapHint(err)
				}
			}
			opts := getOpts()
			return output.WriteResult(cmd.OutOrStdout(),
				output.WriteOpts{JSON: opts.JSON, Quiet: opts.Quiet},
				"Logged", "worklog", wid, raw)
		},
	}

	cmd.Flags().StringVar(&hours, "hours", "", "worked hours ending now (e.g. 1.5)")
	cmd.Flags().StringVar(&from, "from", "", "interval start \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&to, "to", "", "interval end \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&dateFlag, "date", "", "calendar date of the entry (YYYY-MM-DD)")
	cmd.Flags().StringVar(&note, "note", "", "follow-up comment posted after the entry")
	cmd.Flags().IntVar(&workType, "work-type", 0, "work type directory entry key")
	cmd.Flags().BoolVar(&refresh, "refresh-worklog-meta", false, "force worklog field re-discovery")
	return cmd
}

func newListCmd(getClient ClientFunc, getOpts func() Options, getMeta MetaFunc) *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "list <task-id>",
		Short: "List logged work time on a task",
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
			meta, err := getMeta(cmd.Context(), c, refresh)
			if err != nil {
				return planfix.WrapHint(err)
			}
			rows, raw, err := c.ListWorklog(cmd.Context(), id, meta)
			if err != nil {
				return planfix.WrapHint(err)
			}
			opts := getOpts()
			switch {
			case opts.JSON:
				return output.JSON(cmd.OutOrStdout(), raw)
			case opts.Quiet:
				output.Table(cmd.OutOrStdout(), nil, worklogRows(rows))
			default:
				output.Table(cmd.OutOrStdout(),
					[]string{"DATE", "INTERVAL", "HOURS", "WORK TYPE", "AUTHOR"},
					worklogRows(rows))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh-worklog-meta", false, "force worklog field re-discovery")
	return cmd
}

// worklogRows renders WorklogRows as table lines.
func worklogRows(rows []planfix.WorklogRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, []string{
			r.Date,
			r.From + "–" + r.To,
			formatHours(r.Hours),
			r.WorkType,
			r.Author,
		})
	}
	return out
}

// formatHours renders hours rounded to two decimals without trailing zeros.
func formatHours(h float64) string {
	return strconv.FormatFloat(math.Round(h*100)/100, 'g', -1, 64)
}

// DefaultMetaFunc returns a MetaFunc backed by an in-memory cache and the
// profile's persisted worklog block. Missing metadata is discovered via
// worklogtag.Discover and saved back into the config profile.
func DefaultMetaFunc(resolveProfile func() (string, *config.Profile, error)) MetaFunc {
	cache := map[string]*config.WorklogMeta{}
	return func(ctx context.Context, c *planfix.Client, refresh bool) (*config.WorklogMeta, error) {
		name, p, err := resolveProfile()
		if err != nil {
			return nil, err
		}
		if !refresh {
			if m, ok := cache[name]; ok {
				return m, nil
			}
			if p != nil && p.Worklog != nil {
				cache[name] = p.Worklog
				return p.Worklog, nil
			}
		}
		meta, err := worklogtag.Discover(ctx, c)
		if err != nil {
			return nil, err
		}
		if err := persistMeta(name, meta); err != nil {
			return nil, err
		}
		cache[name] = meta
		return meta, nil
	}
}

// persistMeta writes the discovered worklog block into the named profile.
func persistMeta(name string, meta *config.WorklogMeta) error {
	path := config.ResolvePath()
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	p, ok := cfg.Profiles[name]
	if !ok || p == nil {
		p = &config.Profile{}
		cfg.Profiles[name] = p
	}
	p.Worklog = meta
	return config.Save(path, cfg)
}
