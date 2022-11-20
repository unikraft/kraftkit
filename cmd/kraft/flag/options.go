package flag

import (
	"fmt"
	"strings"

	"kraftkit.sh/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/packmanager"
)

type Flag struct {
	// Name is for CLI flag and environment variable.
	Name string

	ConfigName string

	// Shorthand is a shorthand letter.
	Shorthand string

	// Value is the default value. It must be filled to determine the flag type.
	Value interface{}

	// Usage explains how to use the flag.
	Usage string

	// Persistent represents if the flag is persistent
	Persistent bool

	// Deprecated represents if the flag is deprecated
	Deprecated bool
}

type FlagGroup interface {
	Name() string
	Flags() []*Flag
}

type Flags struct {
	PsFlagGroup     *PsFlagGroup
	SpecFlagGroup   *SpecFlagGroup
	GlobalFlagGroup *GlobalFlagGroup
}

// Options holds all the runtime configuration
type Options struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	PsOptions
	SpecOptions
	GlobalOptions
}

func bind(cmd *cobra.Command, flag *Flag) error {
	if flag == nil {
		return nil
	} else if flag.Name == "" {
		viper.SetDefault(flag.ConfigName, flag.Value)
		return nil
	}
	if flag.Persistent {
		if err := viper.BindPFlag(flag.ConfigName, cmd.PersistentFlags().Lookup(flag.Name)); err != nil {
			return err
		}
	} else {
		if err := viper.BindPFlag(flag.ConfigName, cmd.Flags().Lookup(flag.Name)); err != nil {
			return err
		}
	}

	return nil
}

func addFlag(cmd *cobra.Command, flag *Flag) {
	if flag == nil || flag.Name == "" {
		return
	}
	var flags *pflag.FlagSet
	if flag.Persistent {
		flags = cmd.PersistentFlags()
	} else {
		flags = cmd.Flags()
	}

	switch v := flag.Value.(type) {
	case int:
		flags.IntP(flag.Name, flag.Shorthand, v, flag.Usage)
	case string:
		flags.StringP(flag.Name, flag.Shorthand, v, flag.Usage)
	case bool:
		flags.BoolP(flag.Name, flag.Shorthand, v, flag.Usage)
	}

	if flag.Deprecated {
		flags.MarkHidden(flag.Name)
	}
}

func getString(flag *Flag) string {
	if flag == nil {
		return ""
	}
	return viper.GetString(flag.ConfigName)
}

func getInt(flag *Flag) int {
	if flag == nil {
		return 0
	}
	return viper.GetInt(flag.ConfigName)
}

func getBool(flag *Flag) bool {
	if flag == nil {
		return false
	}
	return viper.GetBool(flag.ConfigName)
}

func (f *Flags) groups() []FlagGroup {
	var groups []FlagGroup
	// This order affects the usage message, so they are sorted by frequency of use.
	if f.PsFlagGroup != nil {
		groups = append(groups, f.PsFlagGroup)
	}

	return groups
}

func (f *Flags) AddFlags(cmd *cobra.Command) {
	for _, group := range f.groups() {
		for _, flag := range group.Flags() {
			addFlag(cmd, flag)
		}
	}

}

func (f *Flags) Usages(cmd *cobra.Command) string {
	var usages string
	for _, group := range f.groups() {

		flags := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
		lflags := cmd.LocalFlags()
		for _, flag := range group.Flags() {
			if flag == nil || flag.Name == "" {
				continue
			}
			flags.AddFlag(lflags.Lookup(flag.Name))
		}
		if !flags.HasAvailableFlags() {
			continue
		}

		usages += fmt.Sprintf("%s Flags\n", group.Name())
		usages += flags.FlagUsages() + "\n"
	}
	return strings.TrimSpace(usages)
}

func (f *Flags) Bind(cmd *cobra.Command) error {
	for _, group := range f.groups() {
		if group == nil {
			continue
		}
		for _, flag := range group.Flags() {
			if err := bind(cmd, flag); err != nil {
				return xerrors.Errorf("flag groups: %w", err)
			}
		}
	}
	return nil
}

func (f *Flags) ToOptions() Options {

	opts := Options{}

	if f.GlobalFlagGroup != nil {
		opts.GlobalOptions = f.GlobalFlagGroup.ToOptions()
	}

	if f.SpecFlagGroup != nil {
		opts.SpecOptions = f.SpecFlagGroup.ToOptions()
	}

	if f.PsFlagGroup != nil {
		opts.PsOptions = f.PsFlagGroup.ToOptions()
	}

	return opts
}
