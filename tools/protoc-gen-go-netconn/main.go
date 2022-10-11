// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package main

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	extTypes = new(protoregistry.Types)

	emitEmpty            = flag.Bool("emit_empty", false, "render google.protobuf.Empty")
	emitAnyAsGeneric     = flag.Bool("emit_any_as_generic", false, "render google.protobuf.Any as generic")
	emitMessageOptions   = flag.Bool("emit_message_options", false, "render MessageOptions and their set values")
	emitEnumPrefix       = flag.Bool("emit_enum_prefix", false, "render enums with name prefix")
	remapEnumViaJsonName = flag.Bool("remap_enum_via_json_name", false, "recognize 'json_name' enum value option and use as string value for enums")
	mapEnumToMessage     = flag.Bool("map_enum_to_message", false, "create a map between an enum and a known message")
)

// Recursively register all extensions into the provided protoregistry.Types,
// starting with the protoreflect.FileDescriptor and recursing into its
// MessageDescriptors, their nested MessageDescriptors, and so on.
//
// This leverages the fact that both protoreflect.FileDescriptor and
// protoreflect.MessageDescriptor have identical Messages() and Extensions()
// functions in order to recurse through a single function.
//
// See: https://github.com/golang/protobuf/issues/1260
func registerAllExtensions(extTypes *protoregistry.Types, descs interface {
	Messages() protoreflect.MessageDescriptors
	Extensions() protoreflect.ExtensionDescriptors
},
) error {
	mds := descs.Messages()
	for i := 0; i < mds.Len(); i++ {
		registerAllExtensions(extTypes, mds.Get(i))
	}

	xds := descs.Extensions()
	for i := 0; i < xds.Len(); i++ {
		glog.V(1).Infof("Registering extension %s", xds.Get(i).Name())
		if err := extTypes.RegisterExtension(dynamicpb.NewExtensionType(xds.Get(i))); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Parse()
	defer glog.Flush()

	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if err := registerAllExtensions(extTypes, f.Desc); err != nil {
				return err
			}
		}

		opts := Options{
			EmitEmpty:            *emitEmpty,
			EmitMessageOptions:   *emitMessageOptions,
			EmitAnyAsGeneric:     *emitAnyAsGeneric,
			EmitEnumPrefix:       *emitEnumPrefix,
			RemapEnumViaJsonName: *remapEnumViaJsonName,
			MapEnumToMessage:     *mapEnumToMessage,
		}

		for _, name := range gen.Request.FileToGenerate {
			f := gen.FilesByPath[name]

			if len(f.Messages) == 0 && len(f.Services) == 0 && len(f.Enums) == 0 {
				glog.V(1).Infof("Skipping %s, no messages and services", name)
				continue
			}

			glog.V(1).Infof("Processing %s", name)
			glog.V(2).Infof("Generating %s\n", fmt.Sprintf("%s.pb.netconn.go", f.GeneratedFilenamePrefix))

			gf := gen.NewGeneratedFile(fmt.Sprintf("%s.pb.netconn.go", f.GeneratedFilenamePrefix), f.GoImportPath)

			err := ApplyTemplate(gf, f, opts)
			if err != nil {
				gf.Skip()
				gen.Error(err)
				continue
			}
		}

		return nil
	})
}
