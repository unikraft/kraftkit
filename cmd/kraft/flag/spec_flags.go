package flag

var (
	HypervisorFlag = Flag{
		Name:       "hypervisor",
		ConfigName: "hypervisor",
		Shorthand:  "H",
		Value:      "",
		Usage:      "Set the hypervisor driver.",
	}
	ArchFlag = Flag{
		Name:       "arch",
		ConfigName: "arch",
		Shorthand:  "m",
		Value:      "",
		Usage:      "Filter the list by architecture",
	}
	PlatFlag = Flag{
		Name:       "plat",
		ConfigName: "plat",
		Shorthand:  "p",
		Value:      "",
		Usage:      "Filter the list by platform",
	}
)

type SpecFlagGroup struct {
	Hypervisor   *Flag
	Architecture *Flag
	Platform     *Flag
}

type SpecOptions struct {
	Hypervisor   string
	Architecture string
	Platform     string
}

func NewSpecFlagGroup() *SpecFlagGroup {
	return &SpecFlagGroup{
		Hypervisor:   &HypervisorFlag,
		Platform:     &PlatFlag,
		Architecture: &ArchFlag,
	}
}

func (f *SpecFlagGroup) Name() string {
	return "Spec"
}

func (f *SpecFlagGroup) Flags() []*Flag {
	return []*Flag{f.Hypervisor, f.Architecture, f.Platform}
}

func (f *SpecFlagGroup) ToOptions() SpecOptions {
	return SpecOptions{
		Hypervisor:   getString(f.Hypervisor),
		Architecture: getString(f.Architecture),
		Platform:     getString(f.Platform),
	}
}
