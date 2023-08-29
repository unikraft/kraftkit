package schema

#KraftSpec: {
	@jsonschema(schema="http://json-schema.org/draft-04/schema#")

	// declared for backward compatibility, ignored.
	specification: number | string
	name?:         string
	outdir?:       string
	template?:     #template
	unikraft:      #unikraft
	targets?: [...#target]
	libraries?: {
		{[=~"^[a-zA-Z0-9._-]+$" & !~"^()$"]: #library}
		...
	}

	#unikraft: number | string | {
		source?:  string
		version?: number | string
		kconfig?: #list_or_dict
		...
	}

	#template: number | string | {
		source?:  string
		version?: number | string
		...
	}

	#target: {
		name?:         string
		architecture?: string
		platform?:     string
		initrd?:       #initrd
		command?:      #command
		...
	}

	#architecture: null | bool | number | string | {
		source?:  string
		version?: number | string
		kconfig?: #list_or_dict
		...
	}

	#platform: null | bool | number | string | {
		source?:    string
		version?:   number | string
		kconfig?:   #list_or_dict
		pre_up?:    #command
		post_down?: #command
		cpus?:      int | string
		memory?:    int | string
		...
	}

	#library: null | bool | number | string | {
		source?:  string
		version?: number | string
		kconfig?: #list_or_dict
		...
	}

	#volume: {
		type?:   string
		source?: string
		...
	}

	#network: bool | {
		pre_up?:      #command
		post_down?:   #command
		ip?:          string
		gateway?:     string
		netmask?:     string
		interface?:   string
		driver?:      string
		type?:        string
		bridge_name?: string
		...
	}

	#source: string

	#command: string | [...string]

	#list_or_dict: {
		{[=~".+" & !~"^()$"]: null | bool | number | string}
	} | [...string]

	#initrd: {
		output?:   string
		compress?: bool
		format?:   string
		input?:    #list_or_dict
		...
	}
	...
}