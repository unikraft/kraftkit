package flag

var (
	ShowAllFlag = Flag{
		Name:      "all",
		Shorthand: "a",
		Value:     false,
		Usage:     "Show all machines (default shows just running)",
	}
)

type PsFlagGroup struct {
	ShowAll *Flag
}

type PsOptions struct {
	// Command-line arguments
	ShowAll bool
}

func NewPsFlagGroup() *PsFlagGroup {
	return &PsFlagGroup{
		ShowAll: &ShowAllFlag,
	}
}

func (f *PsFlagGroup) Name() string {
	return "PS"
}

func (f *PsFlagGroup) Flags() []*Flag {
	return []*Flag{f.ShowAll}
}

func (f *PsFlagGroup) ToOptions() PsOptions {
	return PsOptions{
		ShowAll: getBool(f.ShowAll),
	}
}
