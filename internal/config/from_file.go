// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft UG.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package config

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

// This type implements a Config interface and represents a config file on disk.
type fileConfig struct {
	ConfigMap
	documentRoot *yaml.Node
}

type HostConfig struct {
	ConfigMap
	Host string
}

func (c *fileConfig) Root() *yaml.Node {
	return c.ConfigMap.Root
}

func (c *fileConfig) Get(key string) (string, error) {
	val, _, err := c.GetWithSource(key)
	return val, err
}

func (c *fileConfig) GetWithSource(key string) (string, string, error) {
	defaultSource := ConfigFile()

	value, err := c.GetStringValue(key)

	var notFound *NotFoundError

	if err != nil && errors.As(err, &notFound) {
		return defaultFor(key), defaultSource, nil
	} else if err != nil {
		return "", defaultSource, err
	}

	return value, defaultSource, nil
}

func (c *fileConfig) GetOrDefault(key string) (val string, err error) {
	val, _, err = c.GetOrDefaultWithSource(key)
	return
}

func (c *fileConfig) GetOrDefaultWithSource(key string) (val string, src string, err error) {
	val, src, err = c.GetWithSource(key)
	if err != nil && val == "" {
		val = c.Default(key)
	}
	return
}

func (c *fileConfig) Default(key string) string {
	return defaultFor(key)
}

func (c *fileConfig) Set(key, value string) error {
	return c.SetStringValue(key, value)
}

func (c *fileConfig) CheckWriteable(key string) error {
	// TODO: check filesystem permissions
	return nil
}

func (c *fileConfig) Write() error {
	mainData := yaml.Node{Kind: yaml.MappingNode}

	nodes := c.documentRoot.Content[0].Content
	for i := 0; i < len(nodes)-1; i += 2 {
		mainData.Content = append(mainData.Content, nodes[i], nodes[i+1])
	}

	mainBytes, err := yaml.Marshal(&mainData)
	if err != nil {
		return err
	}

	filename := ConfigFile()
	return WriteConfigFile(filename, yamlNormalize(mainBytes))
}

func (c *fileConfig) Aliases() (*AliasConfig, error) {
	// The complexity here is for dealing with either a missing or empty aliases
	// key. It's something we'll likely want for other config sections at some
	// point.
	entry, err := c.FindEntry("aliases")
	var nfe *NotFoundError
	notFound := errors.As(err, &nfe)
	if err != nil && !notFound {
		return nil, err
	}

	toInsert := []*yaml.Node{}

	keyNode := entry.KeyNode
	valueNode := entry.ValueNode

	if keyNode == nil {
		keyNode = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "aliases",
		}
		toInsert = append(toInsert, keyNode)
	}

	if valueNode == nil || valueNode.Kind != yaml.MappingNode {
		valueNode = &yaml.Node{
			Kind:  yaml.MappingNode,
			Value: "",
		}
		toInsert = append(toInsert, valueNode)
	}

	if len(toInsert) > 0 {
		newContent := []*yaml.Node{}
		if notFound {
			newContent = append(c.Root().Content, keyNode, valueNode)
		} else {
			for i := 0; i < len(c.Root().Content); i++ {
				if i == entry.Index {
					newContent = append(newContent, keyNode, valueNode)
					i++
				} else {
					newContent = append(newContent, c.Root().Content[i])
				}
			}
		}
		c.Root().Content = newContent
	}

	return &AliasConfig{
		Parent:    c,
		ConfigMap: ConfigMap{Root: valueNode},
	}, nil
}

func yamlNormalize(b []byte) []byte {
	if bytes.Equal(b, []byte("{}\n")) {
		return []byte{}
	}
	return b
}

func defaultFor(key string) string {
	for _, co := range configOptions {
		if co.Key == key {
			return co.DefaultValue
		}
	}
	return ""
}
