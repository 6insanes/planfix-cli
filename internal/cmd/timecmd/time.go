// Package timecmd implements planfix time add|list (worklog).
package timecmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/config"
	"github.com/6insanes/planfix-cli/internal/output"
	"github.com/6insanes/planfix-cli/internal/planfix"
	"github.com/6insanes/planfix-cli/internal/worklogtag"
)

// Options are the global flags the time commands honour.
type Options struct {
	JSON  bool
	Quiet bool
	Plain bool
}

// ClientFunc builds an API client.
type ClientFunc func() (*planfix.Client, error)

// MetaFunc resolves the account's worklog metadata.
type MetaFunc func(ctx context.Context, c *planfix.Client, opts MetaOpts) (*config.WorklogMeta, error)

// MetaOpts selects the worklog data tag and cache behaviour.
type MetaOpts struct {
	// Refresh forces schema re-resolution.
	Refresh bool
	// DataTag overrides the configured pin for one call (id or exact name).
	DataTag string
}

// dateTimeLayout is the --from/--to argument format.
const dateTimeLayout = "2006-01-02 15:04"

// dateLayout is the --date argument format.
const dateLayout = "2006-01-02"

// apiDateLayout is the worklog date format Planfix expects (dd-MM-yyyy).
const apiDateLayout = "02-01-2006"

// timeLayout is the worklog clock-time format (HH:MM).
const timeLayout = "15:04"

// ParseHours parses a positive worked-hours value, rounded to whole
// minutes. Entries span at most one day, so 24 hours or more is rejected.
func ParseHours(s string) (float64, error) {
	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h <= 0 {
		return 0, fmt.Errorf("invalid hours %q: must be a positive number", s)
	}
	h = math.Round(h*60) / 60
	if h <= 0 {
		return 0, fmt.Errorf("invalid hours %q: must be at least one minute", s)
	}
	if h >= 24 {
		return 0, fmt.Errorf("invalid hours %q: worklog entries must be under 24 hours; log one entry per day", s)
	}
	return h, nil
}

// ParseFromTo parses the --from/--to bounds and requires to > from.
// Spans of 24 hours or more are rejected: entries store one HH:MM period
// per date.
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
	if d := t.Sub(f); d >= 24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf(
			"--from %q to --to %q spans %v; worklog entries must be under 24 hours; log one entry per day",
			from, to, d)
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
func buildEntry(hours, from, to, dateFlag string) (planfix.WorklogEntry, error) {
	var e planfix.WorklogEntry

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
		// Whole minutes keep the formatted HH:MM bounds an exact duration apart.
		start := end.Add(-time.Duration(math.Round(h*60)) * time.Minute)
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
	var hours, from, to, dateFlag, note, workType, dataTag string
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
			entry, err := buildEntry(hours, from, to, dateFlag)
			if err != nil {
				return err
			}
			entry.WorkType, entry.Note = workType, note
			c, err := getClient()
			if err != nil {
				return err
			}
			meta, err := getMeta(cmd.Context(), c, MetaOpts{Refresh: refresh, DataTag: dataTag})
			if err != nil {
				return planfix.WrapHint(err)
			}
			wid, raw, err := c.CreateWorklogEntry(cmd.Context(), id, meta, entry)
			if err != nil {
				return planfix.WrapHint(err)
			}
			if note != "" && meta.FieldNote == 0 {
				if _, _, err := c.AddComment(cmd.Context(), id, note, true); err != nil {
					return planfix.WrapHint(fmt.Errorf("worklog %d created; note failed: %w", wid, err))
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
	cmd.Flags().StringVar(&note, "note", "", "work details (the tag's details field, or a comment)")
	cmd.Flags().StringVar(&workType, "work-type", "", "work type: list value name or directory entry key")
	cmd.Flags().StringVar(&dataTag, "data-tag", "", "worklog data tag override (id or exact name)")
	cmd.Flags().BoolVar(&refresh, "refresh-worklog-meta", false, "force worklog field re-resolution")
	return cmd
}

func newListCmd(getClient ClientFunc, getOpts func() Options, getMeta MetaFunc) *cobra.Command {
	var dataTag string
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
			meta, err := getMeta(cmd.Context(), c, MetaOpts{Refresh: refresh, DataTag: dataTag})
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
					[]string{"DATE", "FROM–TO", "HOURS", "WORK TYPE", "NOTE", "AUTHOR"},
					output.StripRows(worklogRows(rows), opts.Plain))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dataTag, "data-tag", "", "worklog data tag override (id or exact name)")
	cmd.Flags().BoolVar(&refresh, "refresh-worklog-meta", false, "force worklog field re-resolution")
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
			r.Note,
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
// profile's persisted worklog block. The data tag comes from the profile's
// worklog.datatag pin (or the per-call --data-tag override); its field
// schema is resolved via worklogtag.Resolve and saved back into the config
// profile.
func DefaultMetaFunc(resolveProfile func() (string, *config.Profile, error)) MetaFunc {
	cache := map[string]*config.WorklogMeta{}
	return func(ctx context.Context, c *planfix.Client, opts MetaOpts) (*config.WorklogMeta, error) {
		name, p, err := resolveProfile()
		if err != nil {
			return nil, err
		}
		if opts.DataTag != "" {
			return worklogtag.Resolve(ctx, c, opts.DataTag)
		}
		pin := ""
		if p != nil && p.Worklog != nil {
			pin = p.Worklog.DataTag
		}
		if !opts.Refresh {
			if m, ok := cache[name]; ok && metaMatchesPin(m, pin) {
				return m, nil
			}
			if p != nil && p.Worklog != nil && metaMatchesPin(p.Worklog, pin) {
				cache[name] = p.Worklog
				return p.Worklog, nil
			}
		}
		meta, err := worklogtag.Resolve(ctx, c, pin)
		if err != nil {
			return nil, err
		}
		meta.DataTag = pin
		if err := persistMeta(name, meta); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save worklog metadata to config: %v\n", err)
		}
		cache[name] = meta
		return meta, nil
	}
}

// metaMatchesPin reports whether cached metadata is resolved for the pinned
// tag: a numeric pin must equal the resolved tag id, a name pin the recorded
// tag name; incomplete (unresolved) metadata never matches.
func metaMatchesPin(m *config.WorklogMeta, pin string) bool {
	if m == nil || pin == "" || m.FieldDate == 0 {
		return false
	}
	if id, err := strconv.Atoi(pin); err == nil {
		return m.DataTagID == id
	}
	return m.DataTag == pin
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
