// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2017, Ryan Armstrong.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cpio

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func TestRead(t *testing.T) {
	f, err := os.Open("testdata/test_svr4_crc.cpio")
	if err != nil {
		t.Fatalf("error opening test file: %v", err)
	}
	defer f.Close()
	r := NewReader(f)
	for {
		_, _, err := r.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Errorf("error moving to next header: %v", err)
			return
		}
		// TODO: validate header fields
	}
}

func TestSVR4CRC(t *testing.T) {
	f, err := os.Open("testdata/test_svr4_crc.cpio")
	if err != nil {
		t.Fatalf("error opening test file: %v", err)
	}
	defer f.Close()
	w := NewHash()
	r := NewReader(f)
	for {
		hdr, _, err := r.Next()
		if err != nil {
			if err != io.EOF {
				t.Errorf("error moving to next header: %v", err)
			}
			return
		}
		if hdr.Mode.IsRegular() {
			w.Reset()
			_, err = io.CopyN(w, r, hdr.Size)
			if err != nil {
				t.Fatalf("error writing to checksum hash: %v", err)
			}
			sum := w.Sum32()
			if sum != hdr.Checksum {
				t.Errorf("expected checksum %v, got %v for %v", hdr.Checksum, sum, hdr.Name)
			}
		}
	}
}

func ExampleNewHash() {
	// Open the cpio archive for reading.
	f, err := os.Open("testdata/test_svr4_crc.cpio")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r := NewReader(f)

	// Iterate through the files in the archive.
	for {
		hdr, _, err := r.Next()
		if err == io.EOF {
			// end of cpio archive
			return
		}
		if err != nil {
			log.Fatal(err)
		}

		// skip symlinks, directories, etc.
		if !hdr.Mode.IsRegular() {
			continue
		}

		// read file into hash
		h := NewHash()
		_, err = io.CopyN(h, r, hdr.Size)
		if err != nil {
			log.Fatal(err)
		}

		// check hash matches header checksum
		sum := h.Sum32()
		if sum == hdr.Checksum {
			fmt.Printf("Checksum OK: %s (%08X)\n", hdr.Name, hdr.Checksum)
		} else {
			fmt.Printf("Checksum FAIL: %s - expected %08X, got %08X\n", hdr.Name, hdr.Checksum, sum)
		}
	}

	// Output:
	// Checksum OK: gophers.txt (00000C98)
	// Checksum OK: readme.txt (00000E3D)
	// Checksum OK: todo.txt (00000A52)
}
