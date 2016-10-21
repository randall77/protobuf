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
	"fmt"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (g *Generator) generateUnmarshalCode(message *Descriptor) {
	g.generateUnmarshalFullCustom(message)
	g.generateUnmarshalTableDriven(message)
}

// Full custom decoder.
func (g *Generator) generateUnmarshalFullCustom(message *Descriptor) {
	typeName := message.TypeName()
	ccTypeName := CamelCaseSlice(typeName)

	g.P("func (m *", ccTypeName, ") MergeFullCustom(b []byte) error {")
	g.In()
	g.P("for len(b) > 0 {")
	g.In()
	g.P("x,n := proto.DecodeVarint(b)")
	g.P("if n == 0 { return proto.ErrInternalBadWireType }")
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

// table-driven decoder
func (g *Generator) generateUnmarshalTableDriven(message *Descriptor) {
	var submessageLinks []string

	typeName := message.TypeName()
	ccTypeName := CamelCaseSlice(typeName)
	g.P("var XXX_Unpack_", ccTypeName, " = proto.UnpackMessageInfo {")
	g.In()
	g.P("Make: func() unsafe.Pointer {")
	g.In()
	g.P("return unsafe.Pointer(new(", ccTypeName, "))")
	g.Out()
	g.P("},")
	g.P("Dense: []proto.UnpackFieldInfo {")
	g.In()
	for _, field := range message.Field {
		g.P(int(field.GetNumber()), ": {")
		g.In()
		fname := CamelCase(field.GetName())

		if field.OneofIndex != nil {
			odp := message.OneofDecl[int(*field.OneofIndex)]
			dname := "is" + ccTypeName + "_" + CamelCase(odp.GetName())

			g.P("Offset: unsafe.Offsetof(", ccTypeName, "{}.", CamelCase(odp.GetName()), "),")
			// Use generated code for the unpacker for oneofs.
			g.P("Unpack: func(b []byte, f unsafe.Pointer, i *proto.UnpackMessageInfo) []byte {")
			g.In()
			g.P("var v ", ccTypeName, "_", fname)
			var typ string

			switch field.GetType() {
			case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
				typ = "Float64_3"
			case descriptor.FieldDescriptorProto_TYPE_FLOAT:
				typ = "Float32_3"
			case descriptor.FieldDescriptorProto_TYPE_INT64:
				typ = "Int64_3"
			case descriptor.FieldDescriptorProto_TYPE_UINT64:
				typ = "Int64_3"
			case descriptor.FieldDescriptorProto_TYPE_SINT64:
				typ = "Sint64_3"
			case descriptor.FieldDescriptorProto_TYPE_INT32:
				typ = "Int32_3"
			case descriptor.FieldDescriptorProto_TYPE_UINT32:
				typ = "Int32_3"
			case descriptor.FieldDescriptorProto_TYPE_SINT32:
				typ = "Sint64_3"
			case descriptor.FieldDescriptorProto_TYPE_ENUM:
				typ = "Enum_3"
			case descriptor.FieldDescriptorProto_TYPE_BOOL:
				typ = "Bool_3"
			case descriptor.FieldDescriptorProto_TYPE_FIXED64:
				typ = "Fixed64_3"
			case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
				typ = "Fixed64_3"
			case descriptor.FieldDescriptorProto_TYPE_FIXED32:
				typ = "Fixed32_3"
			case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
				typ = "Fixed32_3"
			case descriptor.FieldDescriptorProto_TYPE_STRING:
				typ = "String_3"
			case descriptor.FieldDescriptorProto_TYPE_BYTES:
				typ = "Bytes"
			case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
				typ = "Message"
			}
			// Wrap value in struct that implements oneof interface.
			// Store in field.
			g.P("b = proto.Unpack", typ, "(b, unsafe.Pointer(&v.", fname, "), i)")
			g.P("*(*", dname, ")(f) = &v")
			g.P("return b")
			g.Out()
			g.P("},")
			g.Out()
			g.P("},")
			continue
		}

		g.P("Offset: unsafe.Offsetof(", ccTypeName, "{}.", fname, "),")

		// Standard fields use a known fixed set of unpackers.
		var suffix string
		switch {
		case field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REPEATED:
			suffix = "_R"
		case message.proto3():
			suffix = "_3"
		default:
			suffix = "_2"
		}
		var fn string
		switch field.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
			fn = "Double"
		case descriptor.FieldDescriptorProto_TYPE_FLOAT:
			fn = "Float"
		case descriptor.FieldDescriptorProto_TYPE_INT64:
			fn = "Int64"
		case descriptor.FieldDescriptorProto_TYPE_UINT64:
			fn = "Int64"
		case descriptor.FieldDescriptorProto_TYPE_SINT64:
			fn = "Sint64"
		case descriptor.FieldDescriptorProto_TYPE_INT32:
			fn = "Int32"
		case descriptor.FieldDescriptorProto_TYPE_UINT32:
			fn = "Int32"
		case descriptor.FieldDescriptorProto_TYPE_SINT32:
			fn = "Sint32"
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			fn = "Enum"
		case descriptor.FieldDescriptorProto_TYPE_BOOL:
			fn = "Bool"
		case descriptor.FieldDescriptorProto_TYPE_FIXED64:
			fn = "Fixed64"
		case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
			fn = "Fixed64"
		case descriptor.FieldDescriptorProto_TYPE_FIXED32:
			fn = "Fixed32"
		case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
			fn = "Fixed32"
		case descriptor.FieldDescriptorProto_TYPE_STRING:
			fn = "String"
		case descriptor.FieldDescriptorProto_TYPE_BYTES:
			fn = "Bytes"
			if suffix != "_R" {
				suffix = "" // Don't need to distinguish proto2 and proto3.
			}
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
			desc := g.ObjectNamed(field.GetTypeName())
			if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
				// The message we just decoded corresponds to a map entry.
				// Generate code to decode into the map entry.
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

				g.P("Unpack: func(b []byte, f unsafe.Pointer, i *proto.UnpackMessageInfo) []byte {")
				g.In()
				g.P("var v ", ccTypeName, "_", fname, "Entry")
				g.P("b = proto.UnpackMessage(b, unsafe.Pointer(&v), i)")
				t := "map[" + keyBase + "]" + valBase
				g.P("m := *(*", t, ")(f)")
				g.P("if m == nil {")
				g.In()
				g.P("m = ", t, "{}")
				g.P("*(*", t, ")(f) = m")
				g.Out()
				g.P("}")
				g.P("m[", keyStar, "v.Key] = ", valStar, "v.Value")
				g.P("return b")
				g.Out()
				g.P("},")
				g.Out()
				g.P("},")
				continue
			}
			fn = "Message"
			if suffix != "_R" {
				suffix = "" // Don't need to distinguish proto2 and proto3.
			}
			// TODO: handle sparse here.
			submessageLinks = append(submessageLinks,
				fmt.Sprintf("XXX_Unpack_%s.Dense[%d].Sub = &XXX_Unpack_%s", ccTypeName, field.GetNumber(), g.TypeName(g.ObjectNamed(field.GetTypeName()))))
		case descriptor.FieldDescriptorProto_TYPE_GROUP:
			fn = "Group"
			if suffix != "_R" {
				suffix = "" // Don't need to distinguish proto2 and proto3.
			}
			// TODO: handle sparse here.
			submessageLinks = append(submessageLinks,
				fmt.Sprintf("XXX_Unpack_%s.Dense[%d].Sub = &XXX_Unpack_%s", ccTypeName, field.GetNumber(), g.TypeName(g.ObjectNamed(field.GetTypeName()))))
		default:
			panic("unknown type for field: " + field.GetName())
		}
		g.P("Unpack: proto.Unpack", fn, suffix, ",")

		g.Out()
		g.P("},")
	}
	g.Out()
	g.P("},")
	g.P("Sparse: nil,")
	// TODO: for tags which are large, put them in the sparse map instead of the dense map.
	if message.proto3() {
		g.P("UnrecognizedOffset: 1,") // proto3 sentinel
	} else {
		g.P("UnrecognizedOffset: unsafe.Offsetof(", ccTypeName, "{}.XXX_unrecognized),")
	}
	g.Out()
	g.P("}")

	// Link up submessage fields.
	// We can't do that directly in the initializer because it generates
	// cycles in the initialization graph.  See https://github.com/golang/go/issues/17533
	// TODO: only use init for actual cycles?  init prevents pruning of unused message descriptors.
	if submessageLinks != nil {
		g.P("func init() {")
		g.In()
		for _, s := range submessageLinks {
			g.P(s)
		}
		g.Out()
		g.P("}")
	}

	// Eventually we would just call this function Unmarshal.
	// Or we could register the proto name -> unpack info mapping, and get
	// rid of this function entirely.
	g.P("func (m *", ccTypeName, ") MergeTableDriven(b []byte) error {")
	g.In()
	g.P("return proto.UnmarshalMsg(b, unsafe.Pointer(m), &XXX_Unpack_", ccTypeName, ")")
	g.Out()
	g.P("}")

	// build tables using reflect?
	g.P("func (m *", ccTypeName, ") MergeReflectTable(b []byte) error {")
	g.In()
	g.P("return xxx_UnmarshalInfo_", ccTypeName, ".Unmarshal(unsafe.Pointer(m), m, b)")
	g.Out()
	g.P("}")
	g.P("var xxx_UnmarshalInfo_", ccTypeName, " proto.UnmarshalInfo")
}
