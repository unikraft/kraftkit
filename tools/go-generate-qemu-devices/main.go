// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/Masterminds/sprig/v3"
	"github.com/golang/glog"
	"github.com/iancoleman/strcase"
	gofmt "mvdan.cc/gofumpt/format"

	"kraftkit.sh/exec"

	_ "embed"
)

type Option struct {
	Name              string
	Type              string
	Comment           string
	IsSlice           bool
	Default           string
	FirstCharIsNumber bool
}

type Device struct {
	Name    string
	Comment string
	Options []Option
}

type DeviceTemplate struct {
	Devices map[string][]Device
}

var (
	qemuBinary = flag.String("qemu-system-binary", "qemu-system-x86_64", "path to qemu-system-* binary to invoke")

	devicesCategoryMap = DeviceTemplate{
		Devices: make(map[string][]Device),
	}

	matchType    = regexp.MustCompile(`<(.*?)>`)
	matchDefault = regexp.MustCompile(`\(default: (.*?)\)`)

	//go:embed devices.gotmpl
	DevicesTemplate string
)

func main() {
	flag.Parse()
	defer glog.Flush()

	ctx := context.Background()

	// Invoke `qemu-system-* -device help` which will return all the possible
	// devices support by QEMU.
	var devicesRaw bytes.Buffer
	execFindDevices, err := exec.NewProcess(
		*qemuBinary,
		[]string{"-device", "help"},
		exec.WithStdout(bufio.NewWriter(&devicesRaw)),
	)
	if err != nil {
		glog.V(1).Infof("could not prepare invocation of %s: %v", *qemuBinary, err)
		os.Exit(1)
	}

	if err := execFindDevices.StartAndWait(ctx); err != nil {
		glog.V(1).Infof("could not invoke qemu: %v", err)
	}

	// Parse the output from the previous invocation.  The output format is stable
	// and can be used to also determine the category.  Save each device to the
	// named category which is an index in a map of devices.
	var category string
	var devices []Device
	for _, line := range strings.Split(devicesRaw.String(), "\n") {
		// A new blank line indicates we've reached the end of a category
		if line == "" {
			devicesCategoryMap.Devices[category] = devices
			devices = []Device{}
			continue
		}

		// Any line that starts with `name: "` is a device
		if !strings.HasPrefix(line, "name") {
			category = strings.TrimSuffix(line, ":")
			continue
		}

		line = strings.TrimPrefix(line, "name")
		split := strings.Split(line, `"`) // Split using the quotation character
		dev := Device{
			Name:    split[1],
			Comment: strings.TrimPrefix(strings.Join(split[2:], "\""), ", "),
		}

		devices = append(devices, dev)
	}

	// Go through each device and invoke `qemu-system-* -device
	// ${DEVICE_NAME},help` which will return all the list of options for the
	// device.
	for category, devices := range devicesCategoryMap.Devices {
		for i, device := range devices {
			// Perform the invocation
			var optionsRaw bytes.Buffer
			execFindOptions, err := exec.NewProcess(
				ctx,
				*qemuBinary,
				[]string{"-device", fmt.Sprintf("%s,help", device.Name)},
				exec.WithStdout(bufio.NewWriter(&optionsRaw)),
			)
			if err != nil {
				glog.V(1).Infof("could not prepare invocation of %s: %v", *qemuBinary, err)
				os.Exit(1)
			}

			if err := execFindOptions.StartAndWait(); err != nil {
				glog.V(1).Infof("could not invoke qemu: %v", err)
			}

			lines := strings.Split(optionsRaw.String(), "\n")

			// Parse each line from the previous invocation.  Skip the first line,
			// which is the category name.
		lineloop:
			for _, line := range lines[1:] {
				if line == "" { // skip blank lines
					continue
				}

				line = strings.TrimSpace(line)
				split := strings.Split(line, "=")
				name := split[0]
				camel := strcase.ToCamel(name)

				// Check if this option does not already exist
				for _, opt := range devicesCategoryMap.Devices[category][i].Options {
					if strcase.ToCamel(opt.Name) == camel {
						continue lineloop
					}
				}

				option := Option{
					Name: strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "-", "_"),
				}

				if strings.HasSuffix(name, "[0]") {
					name = name[:len(name)-3]
					option.IsSlice = true
				}

				typ := matchType.FindString(split[1])
				switch typ {
				case "<string>":
					option.Type = "string"
				case "<str>":
					option.Type = "string"
				case "<bool>":
					option.Type = "bool"
				case "<uint64>":
					option.Type = "uint64"
				case "<int64>":
					option.Type = "int64"
				case "<uint32>":
					option.Type = "uint32"
				case "<int32>":
					option.Type = "string"
				case "<uint16>":
					option.Type = "uint16"
				case "<int16>":
					option.Type = "int16"
				case "<uint8>":
					option.Type = "uint8"
				case "<int8>":
					option.Type = "int8"
				case "<size>":
					option.Type = "uint32"
				case "<OnOffAuto>":
					option.Type = "QemuDeviceOptOnOffAuto"
				case "<uint>":
					option.Type = "uint"
				case "<int>":
					option.Type = "int"
				default:
					option.Type = "string"
					glog.V(2).Infof("defaulting to string for %s.%s: %s", device.Name, name, typ)
					continue
				}

				if unicode.IsNumber(rune(name[0])) {
					option.FirstCharIsNumber = true
				}

				line = strings.TrimPrefix(line, fmt.Sprintf("%s=%s", name, typ))
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "- ")

				// No need to continue processing if the line is empty
				if line == "" {
					// Add option to device
					devicesCategoryMap.Devices[category][i].Options = append(
						devicesCategoryMap.Devices[category][i].Options,
						option,
					)
					continue
				}

				def := matchDefault.FindString(line)
				comment := strings.TrimSuffix(line, def)
				comment = strings.TrimSpace(comment)
				if comment != "on/off" && comment != "on/off/auto" {
					option.Comment = comment
				}

				def = strings.TrimPrefix(def, "(default: ")
				def = strings.TrimSuffix(def, ")")
				def = strings.TrimPrefix(def, "\"")
				def = strings.TrimSuffix(def, "\"")

				if def != "18446744073709551615" {
					option.Default = def
				}

				// Add option to device
				devicesCategoryMap.Devices[category][i].Options = append(
					devicesCategoryMap.Devices[category][i].Options,
					option,
				)
			}
		}
	}

	var ret bytes.Buffer

	if err := template.Must(
		template.New("devices").
			Funcs(sprig.TxtFuncMap()).
			Parse(DevicesTemplate),
	).Execute(&ret, devicesCategoryMap); err != nil {
		glog.Errorf("could not execute template: %v", err)
		os.Exit(1)
	}

	formatted, err := gofmt.Source(ret.Bytes(), gofmt.Options{})
	if err != nil {
		glog.Errorf("could not execute formatting: %v", err)
		os.Exit(1)
	}

	fmt.Print(string(formatted))
}
