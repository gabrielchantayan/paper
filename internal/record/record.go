package record

import (
	"crypto/rand"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type Type string

const (
	TypeWork Type = "work"
	TypeCut  Type = "cut"
)

type CutKind string

const (
	CutKindToolCall      CutKind = "tool-call"
	CutKindBrokenLink    CutKind = "broken-link"
	CutKindCommand       CutKind = "command"
	CutKindDocumentation CutKind = "documentation"
	CutKindEnvironment   CutKind = "environment"
	CutKindWorkflow      CutKind = "workflow"
	CutKindOther         CutKind = "other"
)

var (
	idPattern   = regexp.MustCompile(`^\d{8}T\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*-[a-f0-9]{4}$`)
	slugPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

type ID string

func ParseID(value string) (ID, error) {
	if !idPattern.MatchString(value) {
		return "", fmt.Errorf("invalid record ID %q", value)
	}
	parsed, err := time.Parse("20060102T150405", value[:15])
	if err != nil || parsed.Format("20060102T150405") != value[:15] {
		return "", fmt.Errorf("invalid record ID timestamp %q", value[:15])
	}
	return ID(value), nil
}

func NewID(createdAt time.Time, summary string) (ID, error) {
	return newID(createdAt, summary, rand.Reader)
}

func newID(createdAt time.Time, summary string, random io.Reader) (ID, error) {
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(summary), "-"), "-")
	if slug == "" {
		slug = "record"
	}
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], "-")
	}
	suffix := make([]byte, 2)
	if _, err := io.ReadFull(random, suffix); err != nil {
		return "", fmt.Errorf("generate record ID: %w", err)
	}
	return ID(fmt.Sprintf("%s-%s-%x", createdAt.Format("20060102T150405"), slug, suffix)), nil
}

type Project struct {
	Name   string `yaml:"name" json:"name"`
	Key    string `yaml:"key" json:"key"`
	Root   string `yaml:"root" json:"root"`
	Remote string `yaml:"remote,omitempty" json:"remote,omitempty"`
}

type Git struct {
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Commit string `yaml:"commit,omitempty" json:"commit,omitempty"`
}

type Common struct {
	ID        ID
	CreatedAt time.Time
	Summary   string
	Tags      []string
	Project   Project
	Directory string
	Git       *Git
	Agent     string
}

type Record interface {
	CommonFields() Common
	RecordType() Type
}

type Work struct {
	Common
	Details string
}

func (work Work) CommonFields() Common { return work.Common }
func (Work) RecordType() Type          { return TypeWork }

type CutFields struct {
	Kind         CutKind
	Description  string
	Attempted    string
	Impact       string
	Workaround   string
	Evidence     string
	SuggestedFix string
}

type OpenCut struct {
	Common
	CutFields
}

func (cut OpenCut) CommonFields() Common { return cut.Common }
func (OpenCut) RecordType() Type         { return TypeCut }

type ResolvedCut struct {
	Common
	CutFields
	ResolvedAt time.Time
	Resolution string
}

func (cut ResolvedCut) CommonFields() Common { return cut.Common }
func (ResolvedCut) RecordType() Type         { return TypeCut }

func ParseKind(value string) (CutKind, error) {
	kind := CutKind(value)
	switch kind {
	case CutKindToolCall, CutKindBrokenLink, CutKindCommand, CutKindDocumentation,
		CutKindEnvironment, CutKindWorkflow, CutKindOther:
		return kind, nil
	default:
		return "", fmt.Errorf("invalid cut kind %q", value)
	}
}

func NewWork(common Common, details string) (Work, error) {
	if err := validateCommon(common); err != nil {
		return Work{}, err
	}
	return Work{Common: common, Details: strings.TrimSpace(details)}, nil
}

func NewOpenCut(common Common, fields CutFields) (OpenCut, error) {
	if err := validateCommon(common); err != nil {
		return OpenCut{}, err
	}
	if _, err := ParseKind(string(fields.Kind)); err != nil {
		return OpenCut{}, err
	}
	fields = trimCutFields(fields)
	if fields.Description == "" {
		return OpenCut{}, fmt.Errorf("description must not be empty")
	}
	return OpenCut{Common: common, CutFields: fields}, nil
}

func Resolve(cut OpenCut, resolvedAt time.Time, resolution string) (ResolvedCut, error) {
	resolution = strings.TrimSpace(resolution)
	if resolution == "" {
		return ResolvedCut{}, fmt.Errorf("resolution must not be empty")
	}
	if resolvedAt.IsZero() {
		return ResolvedCut{}, fmt.Errorf("resolved timestamp must not be empty")
	}
	if resolvedAt.Before(cut.CreatedAt) {
		return ResolvedCut{}, fmt.Errorf("resolved timestamp must not precede created timestamp")
	}
	return ResolvedCut{
		Common: cut.Common, CutFields: cut.CutFields,
		ResolvedAt: resolvedAt, Resolution: resolution,
	}, nil
}

func validateCommon(common Common) error {
	if _, err := ParseID(string(common.ID)); err != nil {
		return err
	}
	if common.CreatedAt.IsZero() {
		return fmt.Errorf("created timestamp must not be empty")
	}
	if string(common.ID)[:15] != common.CreatedAt.Format("20060102T150405") {
		return fmt.Errorf("record ID timestamp does not match created_at")
	}
	if strings.TrimSpace(common.Summary) == "" {
		return fmt.Errorf("summary must not be empty")
	}
	if strings.TrimSpace(common.Project.Name) == "" || strings.TrimSpace(common.Project.Key) == "" || strings.TrimSpace(common.Project.Root) == "" {
		return fmt.Errorf("project name, key, and root must not be empty")
	}
	return nil
}

func trimCutFields(fields CutFields) CutFields {
	fields.Description = strings.TrimSpace(fields.Description)
	fields.Attempted = strings.TrimSpace(fields.Attempted)
	fields.Impact = strings.TrimSpace(fields.Impact)
	fields.Workaround = strings.TrimSpace(fields.Workaround)
	fields.Evidence = strings.TrimSpace(fields.Evidence)
	fields.SuggestedFix = strings.TrimSpace(fields.SuggestedFix)
	return fields
}
