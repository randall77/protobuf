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

import "github.com/golang/protobuf/protoc-gen-go/descriptor"

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
			if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
				// skip map data for now.
				g.P("x, n = proto.DecodeVarint(b)")
				g.P("if n == 0 { return proto.ErrInternalBadWireType }")
				g.P("b = b[n:]")
				g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
				g.P("b = b[x:]")
				g.Out()
				continue
			}
			g.P("x, n = proto.DecodeVarint(b)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("b = b[n:]")
			g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
			g.P("var v ", g.TypeName(desc))
			g.P("if err := v.MergeFullCustom(b[:x]); err != nil { return err }")
			g.P("b = b[x:]")
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

		// Which field it should be written to.
		fname := CamelCase(field.GetName())

		if field.OneofIndex != nil {
			odp := message.OneofDecl[int(*field.OneofIndex)]
			g.P("w := ", ccTypeName, "_", fname, "{", fname, ":v}")
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
	g.P("  return proto.ErrInternalBadWireType")
	// TODO: instead of returning an error, do:
	// b = proto.ParseUnknown(x, &m.XXX_unrecognized, b)
	//  switch on wire type (x&7), put varint.encode(x) + data in &m.XXX_unrecognized
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
		if field.OneofIndex != nil {
			// Treat oneof data as unrecognized
			continue
		}
		if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE {
			desc := g.ObjectNamed(field.GetTypeName())
			if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
				// Treat map data as unrecognized
				continue
			}
		}

		g.P(int(field.GetNumber()), ": {")
		g.In()
		fname := CamelCase(field.GetName())
		g.P("Offset: unsafe.Offsetof(", ccTypeName, "{}.", fname, "),")
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
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
			fn = "Message"
			if suffix != "_R" {
				suffix = "" // Don't need to distinguish proto2 and proto3.
			}
			g.P("Sub: &XXX_Unpack_", g.TypeName(g.ObjectNamed(field.GetTypeName())), ",")
		case descriptor.FieldDescriptorProto_TYPE_GROUP:
			fn = "Group"
			if suffix != "_R" {
				suffix = "" // Don't need to distinguish proto2 and proto3.
			}
			g.P("Sub: &XXX_Unpack_", g.TypeName(g.ObjectNamed(field.GetTypeName())), ",")
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

	// Eventually we would just call this function Unmarshal.
	// Or we could register the proto name -> unpack info mapping, and get
	// rid of this function entirely.
	g.P("func (m *", ccTypeName, ") MergeTableDriven(b []byte) error {")
	g.In()
	g.P("return proto.UnmarshalMsg(b, unsafe.Pointer(m), &XXX_Unpack_", ccTypeName, ")")
	g.Out()
	g.P("}")
}
