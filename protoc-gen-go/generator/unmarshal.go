// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2016 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

/*
Experiment for other techniques for unmarshaling a protobuf.
*/

package generator

import (
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (g *Generator) generateUnmarshalCode(message *Descriptor) {
	g.generateUnmarshalFullCustom(message)
	g.generateUnmarshalReflectTable(message)
}

// Full custom decoder.
func (g *Generator) generateUnmarshalFullCustom(message *Descriptor) {
	typeName := message.TypeName()
	ccTypeName := CamelCaseSlice(typeName)

	g.P("func (m *", ccTypeName, ") MergeFullCustom(b []byte) error {")
	g.In()
	g.P("for len(b) > 0 {")
	g.In()

	if false {
		g.P("x,n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
	} else {
		// Inline 1 and 2 byte decodes.
		// TODO: if all tags < 16, don't bother with the 2 byte case.
		g.P("var x uint64")
		g.P("var n int")
		g.P("if b[0] < 128 {")
		g.In()
		g.P("x = uint64(b[0])")
		g.P("n = 1")
		g.Out()
		g.P("} else if len(b) >= 2 && b[1] < 128 {")
		g.In()
		g.P("x = uint64(b[0]&0x7f) + uint64(b[1])<<7")
		g.P("n = 2")
		g.Out()
		g.P("} else {")
		g.In()
		g.P("x,n = proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.Out()
		g.P("}")
	}

	// TODO: instead of doing DecodeVarint and switching on the result,
	// switch directly on b[0], then b[1], ...

	g.P("b = b[n:]")
	g.P("switch x>>3 {")
	g.In()
	for _, field := range message.Field {
		// Which field we're updating.
		// Note: not the field name for enums, that is fixed below.
		fname := CamelCase(field.GetName())

		g.P("case ", int(field.GetNumber()), ":")
		g.In()
		// parse value out of protocol buffer.
		pointer := false
		switch field.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
			g.P("if len(b) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := math.Float64frombits(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)")
			g.P("b = b[8:]")
		case descriptor.FieldDescriptorProto_TYPE_FLOAT:
			g.P("if len(b) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := math.Float32frombits(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)")
			g.P("b = b[4:]")
		case descriptor.FieldDescriptorProto_TYPE_INT64:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := int64(x)")
		case descriptor.FieldDescriptorProto_TYPE_UINT64:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := uint64(x)")
		case descriptor.FieldDescriptorProto_TYPE_SINT64:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := int64(x>>1) ^ int64(x)<<63>>63")
		case descriptor.FieldDescriptorProto_TYPE_INT32:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := int32(x)")
		case descriptor.FieldDescriptorProto_TYPE_UINT32:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := uint32(x)")
		case descriptor.FieldDescriptorProto_TYPE_SINT32:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := int32(x>>1) ^ int32(x)<<31>>31")
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("v := ", g.TypeName(g.ObjectNamed(field.GetTypeName())), "(x)")
		case descriptor.FieldDescriptorProto_TYPE_BOOL:
			g.P("if len(b) == 0 { return proto.ErrInternalBadWireType }")
			g.P("v := false")
			g.P("if b[0] != 0 {")
			g.In()
			g.P("v = true")
			g.Out()
			g.P("}")
			g.P("b = b[1:]")
		case descriptor.FieldDescriptorProto_TYPE_FIXED64:
			g.P("if len(b) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56")
			g.P("b = b[8:]")
		case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
			g.P("if len(b) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 | int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7])<<56")
			g.P("b = b[8:]")
		case descriptor.FieldDescriptorProto_TYPE_FIXED32:
			g.P("if len(b) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24")
			g.P("b = b[4:]")
		case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
			g.P("if len(b) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24")
			g.P("b = b[4:]")
		case descriptor.FieldDescriptorProto_TYPE_STRING:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
			g.P("v := string(b[:x])")
			g.P("b = b[x:]")
		case descriptor.FieldDescriptorProto_TYPE_BYTES:
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
			g.P("v := make([]byte, x)")
			g.P("copy(v, b)")
			g.P("b = b[x:]")
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
			desc := g.ObjectNamed(field.GetTypeName())
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
			g.P("var v ", g.TypeName(desc))
			g.P("if err := v.MergeFullCustom(b[:x]); err != nil { return err }")
			g.P("b = b[x:]")
			if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
				// The message we just decoded corresponds to a map entry.
				// Generate code to move the data from the message to the map.
				// This gets ugly because of the smattering of * most places,
				// but not everywhere.
				keyField, valField := d.Field[0], d.Field[1]
				keyType, _ := g.GoType(d, keyField)
				valType, _ := g.GoType(d, valField)
				var keyBase, valBase string
				if message.proto3() {
					keyBase = keyType
					valBase = valType
				} else {
					keyBase = strings.TrimPrefix(keyType, "*")
					switch *valField.Type {
					case descriptor.FieldDescriptorProto_TYPE_ENUM:
						valBase = strings.TrimPrefix(valType, "*")
						g.RecordTypeUse(valField.GetTypeName())
					case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
						valBase = valType
						g.RecordTypeUse(valField.GetTypeName())
					default:
						valBase = strings.TrimPrefix(valType, "*")
					}
				}
				keyStar := "*"
				if keyType == keyBase {
					keyStar = ""
				}
				valStar := "*"
				if valType == valBase {
					valStar = ""
				}
				g.P("if m.", fname, " == nil {")
				g.In()
				g.P("m.", fname, " = map[", keyBase, "]", valBase, "{}")
				g.P("}")
				g.Out()
				g.P("m.", CamelCase(field.GetName()), "[", keyStar, "v.Key] = ", valStar, "v.Value")
				g.Out()
				continue
			}
			pointer = true
		case descriptor.FieldDescriptorProto_TYPE_GROUP:
			g.P("i, j := proto.FindEndGroup(b)")
			g.P("if i < 0 { return proto.ErrInternalBadWireType }")
			g.P("var v ", g.TypeName(g.ObjectNamed(field.GetTypeName())))
			g.P("if err := v.MergeFullCustom(b[:i]); err != nil { return err }")
			g.P("b = b[j:]")
			pointer = true
		default:
			panic("unknown type for field: " + field.GetName())
		}

		// Value to write.
		v := "v"

		if field.OneofIndex != nil {
			odp := message.OneofDecl[int(*field.OneofIndex)]
			if pointer {
				g.P("w := ", ccTypeName, "_", fname, "{", fname, ":&v}")
			} else {
				g.P("w := ", ccTypeName, "_", fname, "{", fname, ":v}")
			}
			v = "w"
			pointer = true // oneofs are always pointers
			fname = CamelCase(odp.GetName())
		}

		// Decide if we need to store v or &v.
		if !message.proto3() && field.GetLabel() != descriptor.FieldDescriptorProto_LABEL_REPEATED && field.GetType() != descriptor.FieldDescriptorProto_TYPE_BYTES {
			pointer = true
		}

		if pointer {
			v = "&" + v
		}

		// store value into message structure.
		switch field.GetLabel() {
		case descriptor.FieldDescriptorProto_LABEL_REPEATED:
			g.P("m.", fname, "= append(m.", fname, ", ", v, ")")
		case descriptor.FieldDescriptorProto_LABEL_REQUIRED, descriptor.FieldDescriptorProto_LABEL_OPTIONAL:
			g.P("m.", fname, "= ", v)
		}
		g.Out()
	}
	g.P("default:")
	g.In()
	if message.proto3() {
		g.P("b = proto.SkipUnrecognized(b, x, nil)")
	} else {
		g.P("b = proto.SkipUnrecognized(b, x, &m.XXX_unrecognized)")
	}
	g.Out()
	g.Out()
	g.P("}")
	g.Out()
	g.P("}")
	g.Out()
	g.P("return nil")
	g.P("}")
}

func hack(typ string) (string, string) {
	// ccTypeName can be qualified with a package name. The
	// package name ends up in the middle of the name. This
	// hack extracts the package name and moves it to the
	// front.
	//
	// Before:
	//   XXX_Unpack_ocr_photo.AlignedFeaturesSettings
	// After
	//   ocr_photo.XXX_Unpack_AlignedFeaturesSettings
	parts := strings.SplitN(typ, ".", 2)
	if len(parts) == 2 {
		return strings.TrimPrefix(parts[0], "XXX_Unpack_") + ".", parts[1]
	}
	return "", typ
}

func (g *Generator) generateUnmarshalReflectTable(message *Descriptor) {
	typeName := message.TypeName()
	ccTypePkg, ccTypeName := hack(CamelCaseSlice(typeName))

	g.P("func (m *", ccTypePkg+ccTypeName, ") MergeReflectTable(b []byte) error {")
	g.In()
	g.P("return proto.UnmarshalReflect(m, b, &", ccTypePkg, "xxx_unmarshalInfoPtr_", ccTypeName, ")")
	g.Out()
	g.P("}")
	g.P("var ", ccTypePkg, "xxx_unmarshalInfoPtr_", ccTypeName, " *proto.UnmarshalInfo")
}
