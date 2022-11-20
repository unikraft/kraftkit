package flag

import (
	"github.com/spf13/cobra"
)

var (
	NoPromtFlag = Flag{
		Name:       "no-promt",
		ConfigName: "no-promt",
		Shorthand:  "np",
		Value:      false,
		Usage:      "Do not prompt for interaction (assumes no)",
		Persistent: true,
	}
	AutomaticallyApproveFlag = Flag{
		Name:       "automatically-approve",
		ConfigName: "automatically-approve",
		Shorthand:  "aa",
		Value:      false,
		Usage:      "Automatically approve any yes/no prompts during execution",
		Persistent: true,
	}
	QuietFlag = Flag{
		Name:       "quiet",
		ConfigName: "quiet",
		Shorthand:  "q",
		Value:      false,
		Usage:      "suppress progress bar and log output",
		Persistent: true,
	}
	LongFlag = Flag{
		Name:       "long",
		ConfigName: "long",
		Shorthand:  "l",
		Value:      false,
		Usage:      "Show more information",
		Persistent: true,
	}
	DebugFlag = Flag{
		Name:       "debug",
		ConfigName: "debug",
		Shorthand:  "d",
		Value:      false,
		Usage:      "debug mode",
		Persistent: true,
	}
	ShowTimestampsFlag = Flag{
		Name:       "show-timestamps",
		ConfigName: "show-timestamps",
		Shorthand:  "st",
		Value:      false,
		Usage:      "Shows timestamps",
		Persistent: true,
	}
)

// GlobalFlagGroup composes global flags
type GlobalFlagGroup struct {
	NoPromt              *Flag
	AutomaticallyApprove *Flag
	Quiet                *Flag
	Long                 *Flag
	Debug                *Flag
	ShowTimestamps       *Flag
}

// GlobalOptions defines flags and other configuration parameters for all the subcommands
type GlobalOptions struct {
	NoPromt              bool
	AutomaticallyApprove bool
	Quiet                bool
	Long                 bool
	Debug                bool
	ShowTimestamps       bool
}

func NewGlobalFlagGroup() *GlobalFlagGroup {
	return &GlobalFlagGroup{
		NoPromt:              &NoPromtFlag,
		AutomaticallyApprove: &AutomaticallyApproveFlag,
		Quiet:                &QuietFlag,
		Long:                 &LongFlag,
		Debug:                &DebugFlag,
		ShowTimestamps:       &ShowTimestampsFlag,
	}
}

func (f *GlobalFlagGroup) flags() []*Flag {
	return []*Flag{f.NoPromt, f.AutomaticallyApprove, f.Quiet, f.Long, f.Debug, f.ShowTimestamps}
}

func (f *GlobalFlagGroup) AddFlags(cmd *cobra.Command) {
	for _, flag := range f.flags() {
		addFlag(cmd, flag)
	}
}

func (f *GlobalFlagGroup) Bind(cmd *cobra.Command) error {
	for _, flag := range f.flags() {
		if err := bind(cmd, flag); err != nil {
			return err
		}
	}
	return nil
}

func (f *GlobalFlagGroup) ToOptions() GlobalOptions {
	return GlobalOptions{
		NoPromt:              getBool(f.NoPromt),
		AutomaticallyApprove: getBool(f.AutomaticallyApprove),
		Quiet:                getBool(f.Quiet),
		Long:                 getBool(f.Long),
		Debug:                getBool(f.Debug),
		ShowTimestamps:       getBool(f.ShowTimestamps),
	}
}
