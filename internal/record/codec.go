package record

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	ID         string   `yaml:"id"`
	Type       string   `yaml:"type"`
	CreatedAt  string   `yaml:"created_at"`
	Status     string   `yaml:"status,omitempty"`
	ResolvedAt string   `yaml:"resolved_at,omitempty"`
	Summary    string   `yaml:"summary"`
	Kind       string   `yaml:"kind,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	Project    Project  `yaml:"project"`
	Directory  string   `yaml:"directory,omitempty"`
	Git        *Git     `yaml:"git,omitempty"`
	Agent      string   `yaml:"agent,omitempty"`
}

var sectionNames = map[string]bool{
	"Details": true, "Description": true, "Attempted": true, "Impact": true,
	"Workaround": true, "Evidence": true, "Suggested Fix": true, "Resolution": true,
}

func Encode(value Record) ([]byte, error) {
	common := value.CommonFields()
	meta := frontmatter{
		ID: string(common.ID), Type: string(value.RecordType()),
		CreatedAt: common.CreatedAt.Format(time.RFC3339), Summary: common.Summary,
		Tags: common.Tags, Project: common.Project, Directory: common.Directory,
		Git: common.Git, Agent: common.Agent,
	}
	sections := make([]section, 0, 8)
	switch typed := value.(type) {
	case Work:
		if typed.Details != "" {
			sections = append(sections, section{"Details", typed.Details})
		}
	case OpenCut:
		meta.Status = "open"
		meta.Kind = string(typed.Kind)
		sections = cutSections(typed.CutFields)
	case ResolvedCut:
		meta.Status = "resolved"
		meta.Kind = string(typed.Kind)
		meta.ResolvedAt = typed.ResolvedAt.Format(time.RFC3339)
		sections = append(cutSections(typed.CutFields), section{"Resolution", typed.Resolution})
	default:
		return nil, fmt.Errorf("unsupported record type %T", value)
	}

	encoded, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	var output bytes.Buffer
	output.WriteString("---\n")
	output.Write(encoded)
	output.WriteString("---\n")
	for _, section := range sections {
		fmt.Fprintf(&output, "\n## %s\n\n%s\n", section.name, escapeSectionContent(section.content))
	}
	return output.Bytes(), nil
}

func Parse(data []byte) (Record, error) {
	frontmatterData, body, err := splitDocument(data)
	if err != nil {
		return nil, err
	}
	var meta frontmatter
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatterData))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("parse frontmatter: multiple YAML documents are not allowed")
		}
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	common, err := parseCommon(meta)
	if err != nil {
		return nil, err
	}
	sections, err := parseSections(body)
	if err != nil {
		return nil, err
	}

	switch Type(meta.Type) {
	case TypeWork:
		if meta.Status != "" || meta.Kind != "" || meta.ResolvedAt != "" {
			return nil, fmt.Errorf("work record contains cut-only metadata")
		}
		if err := allowSections(sections, "Details"); err != nil {
			return nil, err
		}
		return NewWork(common, sections["Details"])
	case TypeCut:
		kind, err := ParseKind(meta.Kind)
		if err != nil {
			return nil, err
		}
		if err := allowSections(sections, "Description", "Attempted", "Impact", "Workaround", "Evidence", "Suggested Fix", "Resolution"); err != nil {
			return nil, err
		}
		fields := CutFields{
			Kind: kind, Description: sections["Description"], Attempted: sections["Attempted"],
			Impact: sections["Impact"], Workaround: sections["Workaround"],
			Evidence: sections["Evidence"], SuggestedFix: sections["Suggested Fix"],
		}
		open, err := NewOpenCut(common, fields)
		if err != nil {
			return nil, err
		}
		switch meta.Status {
		case "open":
			if meta.ResolvedAt != "" || sections["Resolution"] != "" {
				return nil, fmt.Errorf("open cut contains resolution data")
			}
			return open, nil
		case "resolved":
			resolvedAt, err := time.Parse(time.RFC3339, meta.ResolvedAt)
			if err != nil {
				return nil, fmt.Errorf("invalid resolved_at: %w", err)
			}
			return Resolve(open, resolvedAt, sections["Resolution"])
		default:
			return nil, fmt.Errorf("invalid cut status %q", meta.Status)
		}
	default:
		return nil, fmt.Errorf("invalid record type %q", meta.Type)
	}
}

func parseCommon(meta frontmatter) (Common, error) {
	id, err := ParseID(meta.ID)
	if err != nil {
		return Common{}, err
	}
	createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
	if err != nil {
		return Common{}, fmt.Errorf("invalid created_at: %w", err)
	}
	common := Common{
		ID: id, CreatedAt: createdAt, Summary: strings.TrimSpace(meta.Summary), Tags: meta.Tags,
		Project: meta.Project, Directory: meta.Directory, Git: meta.Git, Agent: meta.Agent,
	}
	if err := validateCommon(common); err != nil {
		return Common{}, err
	}
	return common, nil
}

func splitDocument(data []byte) ([]byte, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", fmt.Errorf("record must begin with YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("record frontmatter is not closed")
	}
	end += 4
	return []byte(text[4:end]), text[end+5:], nil
}

type section struct{ name, content string }

func cutSections(fields CutFields) []section {
	values := []section{
		{"Description", fields.Description}, {"Attempted", fields.Attempted},
		{"Impact", fields.Impact}, {"Workaround", fields.Workaround},
		{"Evidence", fields.Evidence}, {"Suggested Fix", fields.SuggestedFix},
	}
	result := make([]section, 0, len(values))
	for _, value := range values {
		if value.content != "" {
			result = append(result, value)
		}
	}
	return result
}

func parseSections(body string) (map[string]string, error) {
	sections := make(map[string]string)
	var current string
	var content []string
	flush := func() error {
		if current == "" {
			return nil
		}
		value := strings.TrimSpace(strings.Join(content, "\n"))
		if value == "" {
			return fmt.Errorf("section %q must not be empty", current)
		}
		sections[current] = value
		return nil
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") && sectionNames[strings.TrimSpace(strings.TrimPrefix(line, "## "))] {
			if err := flush(); err != nil {
				return nil, err
			}
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if _, exists := sections[current]; exists {
				return nil, fmt.Errorf("duplicate section %q", current)
			}
			content = nil
			continue
		}
		if current == "" {
			if strings.TrimSpace(line) != "" {
				return nil, fmt.Errorf("content outside a named section")
			}
			continue
		}
		if strings.HasPrefix(line, `\\## `) {
			line = strings.TrimPrefix(line, `\`)
		} else if strings.HasPrefix(line, `\## `) && sectionNames[strings.TrimSpace(strings.TrimPrefix(line, `\## `))] {
			line = strings.TrimPrefix(line, `\`)
		}
		content = append(content, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return sections, nil
}

func escapeSectionContent(content string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, `\## `) {
			lines[index] = `\` + line
		} else if strings.HasPrefix(line, "## ") && sectionNames[strings.TrimSpace(strings.TrimPrefix(line, "## "))] {
			lines[index] = `\` + line
		}
	}
	return strings.Join(lines, "\n")
}

func allowSections(sections map[string]string, allowed ...string) error {
	set := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		set[name] = true
	}
	for name := range sections {
		if !set[name] {
			return fmt.Errorf("section %q is not valid for this record", name)
		}
	}
	return nil
}
