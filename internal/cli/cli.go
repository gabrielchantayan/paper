package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gabrielchantayan/paper/internal/agent"
	"github.com/gabrielchantayan/paper/internal/config"
	"github.com/gabrielchantayan/paper/internal/project"
	"github.com/gabrielchantayan/paper/internal/record"
	"github.com/gabrielchantayan/paper/internal/store"
)

const usage = `Paper records completed work and actionable friction across projects.

Usage:
  paper init [--vault PATH]
  paper log --summary TEXT [--details TEXT|-] [--tag TAG] [--agent NAME]
  paper cut --summary TEXT --description TEXT|- [--kind KIND] [--tag TAG]
  paper cuts [--project PROJECT] [--format text|json]
  paper history [--since DATE] [--until DATE] [--project PROJECT] [--all]
  paper show ID [--format text|json]
  paper resolve ID --resolution TEXT|-
  paper version
  paper agent install|uninstall claude|opencode|kimi|pi|all
  paper agent doctor [claude|opencode|kimi|pi|all]
`

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

const agentUsage = `Manage Paper's user-level agent skills.

Usage:
  paper agent install TARGET
  paper agent uninstall TARGET
  paper agent doctor [TARGET]

Targets:
  claude    Claude Code
  opencode  OpenCode
  kimi      Kimi Code
  pi        Pi
  all       Every supported agent

Examples:
  paper agent install all
  paper agent doctor kimi
  paper agent uninstall pi
`

type stringList []string

func (list *stringList) String() string { return strings.Join(*list, ",") }
func (list *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value must not be empty")
	}
	*list = append(*list, value)
	return nil
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("command is required")
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return nil
	case "version", "--version":
		_, err := fmt.Fprintf(stdout, "paper %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildDate)
		return err
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "log":
		return runLog(ctx, args[1:], stdin, stdout, stderr)
	case "cut":
		return runCut(ctx, args[1:], stdin, stdout, stderr)
	case "cuts":
		return runCuts(args[1:], stdout, stderr)
	case "history":
		return runHistory(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "resolve":
		return runResolve(args[1:], stdin, stdout, stderr)
	case "agent":
		return runAgent(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInit(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("init", stderr)
	vault := flags.String("vault", "", "vault path")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	resolved, err := config.Resolve(*vault)
	if err != nil {
		return err
	}
	opened, err := store.Open(resolved)
	if err != nil {
		return err
	}
	path, err := config.Write(resolved)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Initialized Paper vault at %s\nConfig: %s\n", opened.Root(), path)
	return err
}

func runLog(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := newFlags("log", stderr)
	vault, format, projectName := commonFlags(flags)
	summary := flags.String("summary", "", "work summary")
	details := flags.String("details", "", "optional details or - for stdin")
	agentName := flags.String("agent", "", "agent name")
	var tags stringList
	flags.Var(&tags, "tag", "tag; repeatable")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	parsedSummary, err := required(*summary, "summary")
	if err != nil {
		return err
	}
	parsedDetails, err := readValue(*details, "details", stdin)
	if err != nil {
		return err
	}
	common, err := makeCommon(ctx, parsedSummary, appendTag(tags, "paper/work"), *agentName, *projectName)
	if err != nil {
		return err
	}
	work, err := record.NewWork(common, parsedDetails)
	if err != nil {
		return err
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	written, err := stored.Write(work)
	if err != nil {
		return err
	}
	return printCreated(stdout, *format, written)
}

func runCut(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := newFlags("cut", stderr)
	vault, format, projectName := commonFlags(flags)
	summary := flags.String("summary", "", "cut summary")
	description := flags.String("description", "", "description or - for stdin")
	kindValue := flags.String("kind", string(record.CutKindOther), "cut kind")
	attempted := flags.String("attempted", "", "attempted action")
	impact := flags.String("impact", "", "impact")
	workaround := flags.String("workaround", "", "workaround")
	evidence := flags.String("evidence", "", "evidence")
	suggestedFix := flags.String("suggested-fix", "", "suggested fix")
	agentName := flags.String("agent", "", "agent name")
	var tags stringList
	flags.Var(&tags, "tag", "tag; repeatable")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if countStdin(*description, *attempted, *impact, *workaround, *evidence, *suggestedFix) > 1 {
		return fmt.Errorf("only one field may read from stdin")
	}
	parsedSummary, err := required(*summary, "summary")
	if err != nil {
		return err
	}
	values := []*string{description, attempted, impact, workaround, evidence, suggestedFix}
	for index, value := range values {
		parsed, err := readValue(*value, "cut field", stdin)
		if err != nil {
			return err
		}
		*values[index] = parsed
	}
	if _, err := required(*description, "description"); err != nil {
		return err
	}
	kind, err := record.ParseKind(*kindValue)
	if err != nil {
		return err
	}
	common, err := makeCommon(ctx, parsedSummary, appendTag(tags, "paper/cut"), *agentName, *projectName)
	if err != nil {
		return err
	}
	cut, err := record.NewOpenCut(common, record.CutFields{
		Kind: kind, Description: *description, Attempted: *attempted, Impact: *impact,
		Workaround: *workaround, Evidence: *evidence, SuggestedFix: *suggestedFix,
	})
	if err != nil {
		return err
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	written, err := stored.Write(cut)
	if err != nil {
		return err
	}
	return printCreated(stdout, *format, written)
}

func runCuts(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("cuts", stderr)
	vault, format, projectName := commonFlags(flags)
	all := flags.Bool("all", false, "include resolved cuts")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	items, err := stored.Cuts(*all)
	if err != nil {
		return err
	}
	items = filterProject(items, *projectName)
	return printList(stdout, *format, items)
}

func runHistory(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("history", stderr)
	vault, format, projectName := commonFlags(flags)
	sinceValue := flags.String("since", "", "start date YYYY-MM-DD")
	untilValue := flags.String("until", "", "end date YYYY-MM-DD")
	all := flags.Bool("all", false, "show all work")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *all && (*sinceValue != "" || *untilValue != "") {
		return fmt.Errorf("--all cannot be combined with --since or --until")
	}
	now := time.Now()
	var since, until time.Time
	var err error
	if !*all {
		if *sinceValue == "" && *untilValue == "" {
			since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		} else if *sinceValue != "" {
			if since, err = parseDate(*sinceValue, now.Location()); err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
		}
		if *untilValue != "" {
			until, err = parseDate(*untilValue, now.Location())
			if err != nil {
				return fmt.Errorf("invalid --until: %w", err)
			}
			until = until.AddDate(0, 0, 1)
		}
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	items, err := stored.Work()
	if err != nil {
		return err
	}
	items = filterProject(items, *projectName)
	items = filterTime(items, since, until)
	return printList(stdout, *format, items)
}

func runShow(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("show", stderr)
	vault := flags.String("vault", "", "vault path")
	format := flags.String("format", "text", "output format: text or json")
	id, err := parseRecordArgs(flags, args, "show")
	if err != nil {
		return err
	}
	if err := checkFormat(*format); err != nil {
		return err
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	item, err := stored.Get(id)
	if err != nil {
		return err
	}
	if *format == "json" {
		return writeJSON(stdout, toDTO(item))
	}
	_, err = stdout.Write(item.Raw)
	return err
}

func runResolve(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := newFlags("resolve", stderr)
	vault := flags.String("vault", "", "vault path")
	format := flags.String("format", "text", "output format: text or json")
	resolution := flags.String("resolution", "", "resolution or - for stdin")
	id, err := parseRecordArgs(flags, args, "resolve")
	if err != nil {
		return err
	}
	if err := checkFormat(*format); err != nil {
		return err
	}
	parsed, err := readValue(*resolution, "resolution", stdin)
	if err != nil {
		return err
	}
	if _, err := required(parsed, "resolution"); err != nil {
		return err
	}
	stored, err := openStore(*vault)
	if err != nil {
		return err
	}
	resolved, err := stored.Resolve(id, parsed, time.Now())
	if err != nil {
		return err
	}
	return printCreated(stdout, *format, resolved)
}

func runAgent(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		if _, err := fmt.Fprint(stderr, agentUsage); err != nil {
			return err
		}
		return fmt.Errorf("agent requires install, uninstall, or doctor")
	}
	action := args[0]
	if action == "help" || action == "--help" || action == "-h" {
		_, err := fmt.Fprint(stdout, agentUsage)
		return err
	}
	if action != "install" && action != "uninstall" && action != "doctor" {
		if _, err := fmt.Fprint(stderr, agentUsage); err != nil {
			return err
		}
		return fmt.Errorf("unknown agent action %q", action)
	}
	if len(args) == 2 && (args[1] == "help" || args[1] == "--help" || args[1] == "-h") {
		_, err := fmt.Fprint(stdout, agentUsage)
		return err
	}
	targetName := "all"
	if len(args) == 2 {
		targetName = args[1]
	} else if len(args) > 2 || (len(args) == 1 && action != "doctor") {
		if _, err := fmt.Fprint(stderr, agentUsage); err != nil {
			return err
		}
		return fmt.Errorf("agent %s requires one target", action)
	}
	targets, err := agent.ParseTarget(targetName)
	if err != nil {
		if _, writeErr := fmt.Fprint(stderr, agentUsage); writeErr != nil {
			return writeErr
		}
		return err
	}
	if action != "doctor" {
		for _, target := range targets {
			status, err := agent.Doctor(target)
			if err != nil {
				return err
			}
			if status.Modified && !status.Current {
				return fmt.Errorf("refusing to %s modified or unmanaged skill at %s", action, status.Path)
			}
			if action == "install" && status.Installed && status.Present && !status.Current {
				return fmt.Errorf("managed skill at %s is outdated; uninstall it before installing this version", status.Path)
			}
		}
	}
	for _, target := range targets {
		var status agent.Status
		switch action {
		case "install":
			status, err = agent.Install(target)
		case "uninstall":
			status, err = agent.Uninstall(target)
		case "doctor":
			status, err = agent.Doctor(target)
		}
		if err != nil {
			return err
		}
		if err := printAgentStatus(stdout, action, status); err != nil {
			return err
		}
	}
	if action == "install" {
		_, err = fmt.Fprintln(stdout, "Start a new agent session if the skill is not discovered immediately.")
		return err
	}
	return nil
}

func printAgentStatus(output io.Writer, action string, status agent.Status) error {
	state := "not installed"
	if status.Installed {
		state = "installed and current"
	}
	if status.Modified {
		state = "modified or unmanaged"
	} else if status.Installed && !status.Current {
		state = "outdated"
	}

	switch action {
	case "install":
		_, err := fmt.Fprintf(output,
			"Paper skill for %s is installed.\n  Path: %s\n  Logging identity: %s\n\n",
			agent.DisplayName(status.Target), status.Path, agent.AgentName(status.Target))
		return err
	case "uninstall":
		_, err := fmt.Fprintf(output,
			"Paper skill for %s is uninstalled.\n  Previous path: %s\n\n",
			agent.DisplayName(status.Target), status.Path)
		return err
	default:
		_, err := fmt.Fprintf(output,
			"%s\n  Status: %s\n  Path: %s\n  Logging identity: %s\n\n",
			agent.DisplayName(status.Target), state, status.Path, agent.AgentName(status.Target))
		return err
	}
}

func commonFlags(flags *flag.FlagSet) (vault, format, projectName *string) {
	return flags.String("vault", "", "vault path"),
		flags.String("format", "text", "output format: text or json"),
		flags.String("project", "", "project name or key")
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if format := flags.Lookup("format"); format != nil {
		if err := checkFormat(format.Value.String()); err != nil {
			return err
		}
	}
	return nil
}

func parseRecordArgs(flags *flag.FlagSet, args []string, command string) (string, error) {
	var id string
	flagArgs := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if strings.HasPrefix(argument, "-") {
			flagArgs = append(flagArgs, argument)
			if !strings.Contains(argument, "=") {
				index++
				if index >= len(args) {
					return "", fmt.Errorf("flag %s requires a value", argument)
				}
				flagArgs = append(flagArgs, args[index])
			}
			continue
		}
		if id != "" {
			return "", fmt.Errorf("%s requires one record ID", command)
		}
		id = argument
	}
	if err := flags.Parse(flagArgs); err != nil {
		return "", err
	}
	if id == "" || flags.NArg() != 0 {
		return "", fmt.Errorf("%s requires one record ID", command)
	}
	return id, nil
}

func checkFormat(value string) error {
	if value != "text" && value != "json" {
		return fmt.Errorf("invalid format %q; expected text or json", value)
	}
	return nil
}

func openStore(vault string) (store.Store, error) {
	resolved, err := config.Resolve(vault)
	if err != nil {
		return store.Store{}, err
	}
	return store.Open(resolved)
}

func makeCommon(ctx context.Context, summary string, tags []string, agentName, projectName string) (record.Common, error) {
	directory, err := os.Getwd()
	if err != nil {
		return record.Common{}, fmt.Errorf("find current directory: %w", err)
	}
	projectContext, err := project.Discover(ctx, directory, strings.TrimSpace(projectName))
	if err != nil {
		return record.Common{}, err
	}
	createdAt := time.Now()
	id, err := record.NewID(createdAt, summary)
	if err != nil {
		return record.Common{}, err
	}
	return record.Common{
		ID: id, CreatedAt: createdAt, Summary: summary, Tags: tags,
		Project: projectContext.Project, Directory: projectContext.Directory,
		Git: projectContext.Git, Agent: strings.TrimSpace(agentName),
	}, nil
}

func required(value, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--%s is required and must not be empty", name)
	}
	return value, nil
}

func readValue(value, name string, stdin io.Reader) (string, error) {
	if value != "-" {
		return strings.TrimSpace(value), nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read %s from stdin: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func countStdin(values ...string) int {
	count := 0
	for _, value := range values {
		if value == "-" {
			count++
		}
	}
	return count
}

func appendTag(tags []string, requiredTag string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(tags)+1)
	for _, tag := range append([]string{requiredTag}, tags...) {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

type outputRecord struct {
	ID           string         `json:"id"`
	Type         record.Type    `json:"type"`
	Status       string         `json:"status,omitempty"`
	CreatedAt    string         `json:"created_at"`
	ResolvedAt   string         `json:"resolved_at,omitempty"`
	Summary      string         `json:"summary"`
	Kind         record.CutKind `json:"kind,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Project      record.Project `json:"project"`
	Directory    string         `json:"directory,omitempty"`
	Git          *record.Git    `json:"git,omitempty"`
	Agent        string         `json:"agent,omitempty"`
	Details      string         `json:"details,omitempty"`
	Description  string         `json:"description,omitempty"`
	Attempted    string         `json:"attempted,omitempty"`
	Impact       string         `json:"impact,omitempty"`
	Workaround   string         `json:"workaround,omitempty"`
	Evidence     string         `json:"evidence,omitempty"`
	SuggestedFix string         `json:"suggested_fix,omitempty"`
	Resolution   string         `json:"resolution,omitempty"`
	Path         string         `json:"path"`
}

func toDTO(item store.Stored) outputRecord {
	common := item.Record.CommonFields()
	result := outputRecord{
		ID: string(common.ID), Type: item.Record.RecordType(), CreatedAt: common.CreatedAt.Format(time.RFC3339),
		Summary: common.Summary, Tags: common.Tags, Project: common.Project,
		Directory: common.Directory, Git: common.Git, Agent: common.Agent, Path: item.Path,
	}
	switch typed := item.Record.(type) {
	case record.Work:
		result.Details = typed.Details
	case record.OpenCut:
		result.Status = "open"
		setCutFields(&result, typed.CutFields)
	case record.ResolvedCut:
		result.Status = "resolved"
		result.ResolvedAt = typed.ResolvedAt.Format(time.RFC3339)
		result.Resolution = typed.Resolution
		setCutFields(&result, typed.CutFields)
	}
	return result
}

func setCutFields(output *outputRecord, fields record.CutFields) {
	output.Kind = fields.Kind
	output.Description = fields.Description
	output.Attempted = fields.Attempted
	output.Impact = fields.Impact
	output.Workaround = fields.Workaround
	output.Evidence = fields.Evidence
	output.SuggestedFix = fields.SuggestedFix
}

func printCreated(output io.Writer, format string, item store.Stored) error {
	if format == "json" {
		return writeJSON(output, toDTO(item))
	}
	_, err := fmt.Fprintf(output, "%s\t%s\n", item.Record.CommonFields().ID, item.Path)
	return err
}

func printList(output io.Writer, format string, items []store.Stored) error {
	if format == "json" {
		values := make([]outputRecord, len(items))
		for index, item := range items {
			values[index] = toDTO(item)
		}
		return writeJSON(output, values)
	}
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	for _, item := range items {
		common := item.Record.CommonFields()
		status := "done"
		if _, ok := item.Record.(record.OpenCut); ok {
			status = "open"
		} else if _, ok := item.Record.(record.ResolvedCut); ok {
			status = "resolved"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", common.ID, status, common.CreatedAt.Format("2006-01-02 15:04"), common.Project.Name, common.Summary)
	}
	return writer.Flush()
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func filterProject(items []store.Stored, value string) []store.Stored {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	result := items[:0]
	for _, item := range items {
		project := item.Record.CommonFields().Project
		if project.Name == value || project.Key == value {
			result = append(result, item)
		}
	}
	return result
}

func filterTime(items []store.Stored, since, until time.Time) []store.Stored {
	result := items[:0]
	for _, item := range items {
		createdAt := item.Record.CommonFields().CreatedAt
		if !since.IsZero() && createdAt.Before(since) {
			continue
		}
		if !until.IsZero() && !createdAt.Before(until) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func parseDate(value string, location *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, location)
}
