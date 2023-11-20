// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/log"
)

// kraftignore filename
const KraftignoreFileName = ".kraftignore"

type IgnoringFileType string

const (
	Exist    = IgnoringFileType("Exist")
	NotExist = IgnoringFileType("NotExist")
	SkipDir  = IgnoringFileType("SkipDir")
)

// GetKraftIgnoreItems returns file and directory names specified in .kraftignore
func GetKraftIgnoreItems(ctx context.Context, dir string) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return []string{}, err
	}
	kraftignorePath := filepath.Join(cwd, KraftignoreFileName)

	if _, err := os.Stat(kraftignorePath); errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	} else if err != nil {
		return []string{}, err
	}

	kraftignoreFile, err := os.Open(kraftignorePath)
	if err != nil {
		return []string{}, err
	}

	defer func() {
		kraftIgnoreErr := kraftignoreFile.Close()
		if kraftIgnoreErr != nil {
			if err != nil {
				err = fmt.Errorf("%w: %w", err, kraftIgnoreErr)
			} else {
				err = kraftIgnoreErr
			}
		}
	}()

	kraftignoreScanner := bufio.NewScanner(kraftignoreFile)
	kraftignoreScanner.Split(bufio.ScanLines)
	var kraftignoreFileLines, ignoringItems []string

	for kraftignoreScanner.Scan() {
		kraftignoreFileLines = append(kraftignoreFileLines, kraftignoreScanner.Text())
	}

	for lineNum, line := range kraftignoreFileLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		items := findLineItems(line)
		for _, item := range items {
			if item == "" || item == "#" {
				continue
			}

			if hasGlobPatterns(item) {
				log.G(ctx).
					WithField("file", kraftignorePath).
					Warn("contains a glob pattern ", item,
						" at line ", lineNum,
						" which is not supported by Kraftkit")
				continue
			}

			if _, err := os.Stat(filepath.Join(dir, item)); os.IsNotExist(err) {
				log.G(ctx).
					WithField("file", kraftignorePath).
					Warn("contains ", item,
						" at line ", lineNum,
						" which does not exist in the provided rootfs directory")
				continue
			}

			ignoringItems = append(ignoringItems, item)
		}
	}

	return ignoringItems, err
}

// isExistInKraftignoreFile checks if the path exist in .kraftignore
func isExistInKraftignoreFile(internal string, pathInfo fs.DirEntry, kraftignoreItems []string) IgnoringFileType {
	for _, ignoringItem := range kraftignoreItems {
		if internal == ignoringItem {
			if pathInfo.IsDir() {
				return SkipDir
			}
			return Exist
		}
	}
	return NotExist
}

// hasGlobPatterns checks if the item contains glob pattern
func hasGlobPatterns(item string) bool {
	return strings.ContainsAny(item, "*?![{")
}

// findLineItems finds items in a line of .kraftignore
func findLineItems(line string) []string {
	items := strings.Split(line, " ")
	for index := 0; index < len(items); index++ {
		charToFind := ""
		if strings.HasPrefix(items[index], `"`) && !strings.HasSuffix(items[index], `"`) {
			charToFind = `"`
		} else if strings.HasPrefix(items[index], `'`) && !strings.HasSuffix(items[index], `'`) {
			charToFind = `'`
		}

		if len(charToFind) > 0 {
			i := index + 1
			for ; i < len(items) && !strings.HasSuffix(items[i], charToFind); i++ {
				items[index] += " " + items[i]
				items = append(items[:i], items[i+1:]...)
				i--
			}
			items[index] += " " + items[i]
			items = append(items[:i], items[i+1:]...)
		}
		items[index] = strings.Trim(items[index], `"`)
		items[index] = strings.Trim(items[index], `'`)
		items[index] = strings.TrimSpace(items[index])
		items[index] = strings.TrimPrefix(items[index], "../")
		if !strings.HasPrefix(items[index], "./") {
			if !strings.HasPrefix(items[index], "/") {
				items[index] = "/" + items[index]
			}
			items[index] = "." + items[index]
		}
		items[index] = strings.TrimSuffix(items[index], string(filepath.Separator))
	}
	return items
}
