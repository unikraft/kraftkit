// SPDX-License-Identifier: MIT
// Copyright (c) 2019 GitHub Inc.
// Copyright (c) 2022 Unikraft GmbH.
package cmdfactory

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/text"
)

// AnnotationHelpGroup is used to indicate in which help group a command belongs.
const (
	AnnotationHelpGroup  = "help:group"
	AnnotationHelpHidden = "help:hidden"
)

func rootUsageFunc(command *cobra.Command) error {
	command.Printf("Usage:  %s", command.UseLine())

	subcommands := command.Commands()
	if len(subcommands) > 0 {
		command.Print("\n\nAvailable commands:\n")
		for _, c := range subcommands {
			if _, ok := c.Annotations[AnnotationHelpHidden]; ok {
				continue
			}
			command.Printf("  %s\n", c.Name())
		}
		return nil
	}

	flagUsages := command.LocalFlags().FlagUsagesWrapped(80)
	if flagUsages != "" {
		command.Println("\n\nFlags:")
		command.Print(text.Indent(dedent(flagUsages), "  "))
	}

	return nil
}

func rootFlagErrorFunc(_ *cobra.Command, err error) error {
	if err == pflag.ErrHelp {
		return err
	}
	return FlagErrorWrap(err)
}

var hasFailed bool

// HasFailed signals that the main process should exit with non-zero status
func HasFailed() bool {
	return hasFailed
}

// Display helpful error message in case subcommand name was mistyped.
// This matches Cobra's behavior for root command, which Cobra
// confusingly doesn't apply to nested commands.
func nestedSuggestFunc(command *cobra.Command, arg string) {
	command.Printf("unknown command %q for %q\n", arg, command.CommandPath())

	var candidates []string
	if arg == "help" {
		candidates = []string{"--help"}
	} else {
		if command.SuggestionsMinimumDistance <= 0 {
			command.SuggestionsMinimumDistance = 2
		}
		candidates = command.SuggestionsFor(arg)
	}

	if len(candidates) > 0 {
		command.Print("\nDid you mean this?\n")
		for _, c := range candidates {
			command.Printf("\t%s\n", c)
		}
	}

	command.Print("\n")
	_ = rootUsageFunc(command)
}

func isRootCmd(command *cobra.Command) bool {
	return command != nil && !command.HasParent()
}

func traverse(cmd *cobra.Command) []*cobra.Command {
	var cmds []*cobra.Command

	for _, cmd := range cmd.Commands() {
		cmds = append(cmds, cmd)

		if len(cmd.Commands()) > 0 {
			cmds = append(cmds, traverse(cmd)...)
		}
	}

	return cmds
}

func fullname(parent, cmd *cobra.Command) string {
	name := ""

	for {
		name = cmd.Name() + " " + name
		if !cmd.HasParent() || cmd.Parent() == parent {
			break
		}
		cmd = cmd.Parent()
	}

	return strings.TrimSpace(name)
}

func rootHelpFunc(cmd *cobra.Command, args []string) {
	if isRootCmd(cmd.Parent()) && len(args) >= 2 && !set.NewStringSet(args...).ContainsAnyOf("--help", "-h") {
		nestedSuggestFunc(cmd, args[1])
		hasFailed = true
		return
	}

	type helpEntry struct {
		title string
		body  string
	}

	longText := cmd.Long
	if longText == "" {
		longText = cmd.Short
	}

	helpEntries := []helpEntry{}
	if longText != "" {
		helpEntries = append(helpEntries, helpEntry{"", longText})
	}
	helpEntries = append(helpEntries, helpEntry{"USAGE", cmd.UseLine()})

	if len(cmd.Aliases) > 0 {
		helpEntries = append(helpEntries, helpEntry{
			title: "ALIASES",
			body:  strings.Join(cmd.Aliases, " "),
		})
	}

	maxPad := 0
	mapping := make(map[string][]*cobra.Command)

	if len(cmd.Groups()) > 0 {
		for _, c := range traverse(cmd) {
			if c.Short == "" {
				continue
			}
			if _, ok := c.Annotations[AnnotationHelpHidden]; ok {
				continue
			}

			group, ok := c.Annotations[AnnotationHelpGroup]
			if !ok {
				continue
			}

			pad := len(fullname(cmd, c))
			if pad > maxPad {
				maxPad = pad
			}

			mapping[group] = append(mapping[group], c)
		}
	}

	var printableGroups []string
	for _, group := range cmd.Groups() {
		printableGroups = append(printableGroups, group.ID)
	}

	var usages []string
	var ungrouped []*cobra.Command

	// Add "ungrouped" commands together
	for _, c := range cmd.Commands() {
		if c.Short == "" {
			continue
		}
		if _, ok := c.Annotations[AnnotationHelpHidden]; ok {
			continue
		}

		// Ignore if already in a printable group
		if group, ok := c.Annotations[AnnotationHelpGroup]; ok {
			if slices.Contains[[]string](printableGroups, group) {
				continue
			}
		}

		pad := len(fullname(cmd, c))
		if pad > maxPad {
			maxPad = pad
		}

		ungrouped = append(ungrouped, c)
	}

	for _, c := range ungrouped {
		usages = append(usages, rpad(fullname(cmd, c), maxPad+2)+c.Short)
	}

	if len(usages) > 0 {
		helpEntries = append(helpEntries, helpEntry{
			title: "SUBCOMMANDS",
			body:  strings.Join(usages, "\n"),
		})
	}

	for _, group := range cmd.Groups() {
		var usages []string

		for _, c := range mapping[group.ID] {
			usages = append(usages, rpad(fullname(cmd, c), maxPad+2)+c.Short)
		}

		if len(usages) > 0 {
			helpEntries = append(helpEntries, helpEntry{
				title: group.Title,
				body:  strings.Join(usages, "\n"),
			})
		}
	}

	flagUsages := cmd.LocalFlags().FlagUsages()
	if flagUsages != "" {
		helpEntries = append(helpEntries, helpEntry{"FLAGS", dedent(flagUsages)})
	}

	inheritedFlagUsages := cmd.InheritedFlags().FlagUsages()
	if inheritedFlagUsages != "" {
		helpEntries = append(helpEntries, helpEntry{"INHERITED FLAGS", dedent(inheritedFlagUsages)})
	}

	if cmd.Example != "" {
		helpEntries = append(helpEntries, helpEntry{"EXAMPLES", cmd.Example})
	}

	ctx := cmd.Context()
	err := iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	} else {
		defer iostreams.G(ctx).StopPager()
	}

	out := iostreams.G(ctx).Out
	cs := iostreams.G(ctx).ColorScheme()

	for _, e := range helpEntries {
		if e.title != "" && e.body != "" {
			fmt.Fprintln(out, cs.Bold(e.title))
			fmt.Fprintln(out, text.Indent(strings.Trim(e.body, "\r\n"), "  "))
		} else if e.body != "" {
			fmt.Fprintln(out, e.body)
		}

		fmt.Fprintln(out)
	}
}

// rpad adds padding to the right of a string.
func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds ", padding)
	return fmt.Sprintf(template, s)
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		indent := len(l) - len(strings.TrimLeft(l, " "))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	var buf bytes.Buffer
	for _, l := range lines {
		fmt.Fprintln(&buf, strings.TrimPrefix(l, strings.Repeat(" ", minIndent)))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
