// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package xen

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
)

type XenConfig struct {
	Name    string        `xen_tag:"name"`
	Memory  int           `xen-tag:"memory"`
	CPUs    int           `xen-tag:"vcpus"`
	P9      []P9Spec      `xen-tag:"p9"`
	Network []NetworkSpec `xen-tag:"vif"`
	Kernel  string        `xen-tag:"kernel"`
	Ramdisk string        `xen-tag:"ramdisk"`
	Type    string        `xen-tag:"type"`
	Uuid    string        `xen-tag:"uuid"`
	Args    string        `xen-tag:"cmdline"`

	// This is a post Create() assigned value
	DomID int
}

type P9Spec struct {
	Tag           string `xen-tag:"tag"`
	SecurityModel string `xen-tag:"security_model"`
	Path          string `xen-tag:"path"`
	Backend       string `xen-tag:"backend"`
}

type NetworkSpec struct {
	Mac        string `xen-tag:"mac"`
	Ip         string `xen-tag:"ip"`
	Gatewaydev string `xen-tag:"gatewaydev"`
	Bridge     string `xen-tag:"bridge"`
	Script     string `xen-tag:"script"`
}

const (
	XenMemoryScale   = 1024 * 1024
	XenMemoryDefault = 64
	XenCPUsDefault   = 1
	XenTypeDefault   = "pv"
)

type XenOption func(*XenConfig) error

func NewXenConfig(xopts ...XenOption) (*XenConfig, error) {
	xcfg := &XenConfig{}

	for _, xopt := range xopts {
		if err := xopt(xcfg); err != nil {
			return nil, err
		}
	}
	return xcfg, nil
}

func WithCpu(cpu int) XenOption {
	return func(xc *XenConfig) error {
		xc.CPUs = cpu
		return nil
	}
}

func WithMemory(memory int) XenOption {
	return func(xc *XenConfig) error {
		xc.Memory = memory
		return nil
	}
}

func WithName(name string) XenOption {
	return func(xc *XenConfig) error {
		xc.Name = name
		return nil
	}
}

func WithP9(p9 P9Spec) XenOption {
	return func(xc *XenConfig) error {
		if xc.P9 == nil {
			xc.P9 = []P9Spec{}
		}
		xc.P9 = append(xc.P9, p9)
		return nil
	}
}

func WithNetwork(network NetworkSpec) XenOption {
	return func(xc *XenConfig) error {
		if xc.Network == nil {
			xc.Network = []NetworkSpec{}
		}
		xc.Network = append(xc.Network, network)
		return nil
	}
}

func WithKernel(kernel string) XenOption {
	return func(xc *XenConfig) error {
		xc.Kernel = kernel
		return nil
	}
}

func WithRamdisk(ramdisk string) XenOption {
	return func(xc *XenConfig) error {
		xc.Ramdisk = ramdisk
		return nil
	}
}

func WithType(xtype string) XenOption {
	return func(xc *XenConfig) error {
		xc.Type = xtype
		return nil
	}
}

func WithUuid(uuid string) XenOption {
	return func(xc *XenConfig) error {
		xc.Uuid = uuid
		return nil
	}
}

func WithArgs(args string) XenOption {
	return func(xc *XenConfig) error {
		xc.Args = args
		return nil
	}
}
func marshalXenStruct(xenStruct any) ([]byte, error) {
	value := reflect.ValueOf(xenStruct)
	fieldCount := value.NumField()
	config := []byte{}

	for i := 0; i < fieldCount; i++ {
		field := value.Type().Field(i)
		if value.Field(i).IsZero() {
			continue
		}

		xen_tag := field.Tag.Get("xen-tag")
		if xen_tag == "" {
			continue
		}

		if field.Type.Kind() != reflect.Slice && field.Type.Kind() != reflect.Struct {
			config = append(config, []byte(fmt.Sprintf("%s=%v\n", xen_tag, value.Field(i).Interface()))...)
		}

		if field.Type.Kind() == reflect.Slice {
			sliceBytes := make([][]byte, 0)
			for j := 0; j < value.Field(i).Len(); j++ {
				sliceEl := value.Field(i).Index(j)
				itemBytes := []byte{'\''}
				if itemRawBytes, err := marshalXenStruct(sliceEl.Interface()); err == nil {
					itemRawBytes = bytes.ReplaceAll(itemRawBytes, []byte{'\n'}, []byte{','})
					if itemRawBytes[len(itemRawBytes)-1] == ',' {
						itemRawBytes = itemRawBytes[:len(itemRawBytes)-1]
					}
					itemBytes = append(itemBytes, itemRawBytes...)
				}
				sliceBytes = append(sliceBytes, append(itemBytes, '\''))
			}
			config = append(config, []byte(fmt.Sprintf("%s=[%s]\n", xen_tag, bytes.Join(sliceBytes, []byte(","))))...)
		}
	}

	return config, nil
}

func (config *XenConfig) MarshalXenSpec() (text []byte, err error) {
	return marshalXenStruct(*config)
}

func (config *XenConfig) WriteConfigFile(path string) error {
	text, err := config.MarshalXenSpec()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer file.Close()

	if _, err := file.Write(text); err != nil {
		return err
	}

	return nil
}
