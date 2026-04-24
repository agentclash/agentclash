package cmd

import (
	"fmt"
	"io"
	"os"

	survey "github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

type pickerOption struct {
	Label       string
	Description string
	Value       string
}

type interactivePicker interface {
	Select(prompt string, options []pickerOption) (pickerOption, error)
	MultiSelect(prompt string, options []pickerOption, min int) ([]pickerOption, error)
}

var surveyAskOne = survey.AskOne

// isInteractiveTerminal is the inverse of RunContext.IsNonInteractive.
// Kept as a package var so tests can force a specific branch without
// needing a real tty.
var isInteractiveTerminal = func(rc *RunContext) bool {
	return !rc.IsNonInteractive()
}

// defaultTTYAttached is the production implementation of ttyAttached in
// root.go. Lives here so picker.go owns all the tty-detection bits.
func defaultTTYAttached() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

var newInteractivePicker = func() interactivePicker {
	return &surveyPicker{
		in:  os.Stdin,
		out: os.Stdout,
		err: os.Stderr,
	}
}

type surveyPicker struct {
	in  *os.File
	out *os.File
	err io.Writer
}

func (p *surveyPicker) Select(prompt string, options []pickerOption) (pickerOption, error) {
	if len(options) == 0 {
		return pickerOption{}, fmt.Errorf("no options available for %s", prompt)
	}

	normalized := normalizedPickerOptions(options)
	labels := make([]string, 0, len(normalized))
	indexByLabel := make(map[string]int, len(normalized))
	for i, option := range normalized {
		labels = append(labels, option.Label)
		indexByLabel[option.Label] = i
	}

	selection := ""
	promptUI := &survey.Select{
		Message:     prompt,
		Options:     labels,
		PageSize:    pageSizeForOptions(normalized),
		Description: describePickerOption(normalized),
	}
	if err := surveyAskOne(
		promptUI,
		&selection,
		survey.WithValidator(survey.Required),
		survey.WithStdio(p.in, p.out, p.err),
	); err != nil {
		return pickerOption{}, err
	}

	return normalized[indexByLabel[selection]], nil
}

func (p *surveyPicker) MultiSelect(prompt string, options []pickerOption, min int) ([]pickerOption, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options available for %s", prompt)
	}

	normalized := normalizedPickerOptions(options)
	labels := make([]string, 0, len(normalized))
	indexByLabel := make(map[string]int, len(normalized))
	for i, option := range normalized {
		labels = append(labels, option.Label)
		indexByLabel[option.Label] = i
	}

	selections := []string{}
	promptUI := &survey.MultiSelect{
		Message:     prompt,
		Options:     labels,
		PageSize:    pageSizeForOptions(normalized),
		Description: describePickerOption(normalized),
	}
	if err := surveyAskOne(
		promptUI,
		&selections,
		survey.WithValidator(survey.MinItems(min)),
		survey.WithStdio(p.in, p.out, p.err),
	); err != nil {
		return nil, err
	}

	resolved := make([]pickerOption, 0, len(selections))
	for _, selection := range selections {
		resolved = append(resolved, normalized[indexByLabel[selection]])
	}
	return resolved, nil
}

func pageSizeForOptions(options []pickerOption) int {
	if len(options) < 10 {
		return len(options)
	}
	return 10
}

func describePickerOption(options []pickerOption) func(value string, index int) string {
	return func(_ string, index int) string {
		if index < 0 || index >= len(options) {
			return ""
		}
		return options[index].Description
	}
}

func normalizedPickerOptions(options []pickerOption) []pickerOption {
	counts := make(map[string]int, len(options))
	normalized := make([]pickerOption, len(options))
	for i, option := range options {
		counts[option.Label]++
		normalized[i] = option
	}

	seen := make(map[string]int, len(options))
	for i, option := range normalized {
		label := option.Label
		if counts[option.Label] > 1 {
			label = fmt.Sprintf("%s [%s]", option.Label, option.Value)
		}

		seen[label]++
		if seen[label] > 1 {
			label = fmt.Sprintf("%s (%d)", label, seen[label])
		}
		normalized[i].Label = label
	}

	return normalized
}
