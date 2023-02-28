// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
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

package yamlmerger

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// https://stackoverflow.com/a/65784135
func RecursiveMerge(from, into *yaml.Node) error {
	if from.Kind != into.Kind {
		return fmt.Errorf("cannot merge nodes of different kinds")
	}

	switch from.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(from.Content); i += 2 {
			found := false
			for j := 0; j < len(into.Content); j += 2 {
				if nodesEqual(from.Content[i], into.Content[j]) {
					found = true
					if err := RecursiveMerge(from.Content[i+1], into.Content[j+1]); err != nil {
						return fmt.Errorf("at key " + from.Content[i].Value + ": " + err.Error())
					}
					break
				}
			}
			if !found {
				into.Content = append(into.Content, from.Content[i:i+2]...)
			}
		}
	case yaml.ScalarNode:
		// SA4006 these variables represent pointers and are propagated outside of `recursiveMerge`
		into = from //nolint:staticcheck
	case yaml.SequenceNode:
		for _, fromItem := range from.Content {
			foundFrom := false
			for _, intoItem := range into.Content {
				if fromItem.Value == intoItem.Value {
					foundFrom = true
				}
			}
			if !foundFrom {
				into.Content = append(into.Content, fromItem)
			}
		}
	case yaml.DocumentNode:
		err := RecursiveMerge(from.Content[0], into.Content[0])
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("can only merge mapping, sequence and scalar nodes")
	}

	return nil
}

func nodesEqual(l, r *yaml.Node) bool {
	if l.Kind == yaml.ScalarNode && r.Kind == yaml.ScalarNode {
		return l.Value == r.Value
	}

	// panic("equals on non-scalars not implemented!")
	return false
}
