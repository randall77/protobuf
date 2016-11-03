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
	"sort"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (g *Generator) generateUnmarshalCode(message *Descriptor) {
	g.generateUnmarshalFullCustom(message)
	g.generateUnmarshalReflectTable(message)
}

type Entry struct {
	field   *descriptor.FieldDescriptorProto
	message *Descriptor
	packed  bool // encoded as bytes, read repeated entries out of bytes
}

type Case struct {
	// <= 127 cases
	leaf map[int]Entry

	// >= 128 cases
	next map[int]*Case // sub-cases for next byte
}

// Constants that identify the encoding of a value on the wire.
const (
	wireVarint     = 0
	wireFixed64    = 1
	wireBytes      = 2
	wireStartGroup = 3
	wireEndGroup   = 4
	wireFixed32    = 5
)

var topCase Case

func clearCases() {
	topCase = Case{}
}
func addCase(n int, e Entry) {
	addCaseRec(&topCase, n, e)
}
func addCaseRec(c *Case, n int, e Entry) {
	if n < 128 {
		if c.leaf == nil {
			c.leaf = map[int]Entry{}
		}
		c.leaf[n] = e
		return
	}
	if c.next == nil {
		c.next = map[int]*Case{}
	}
	i := 128 + n&0x7f
	c.next[i] = &Case{}
	addCaseRec(c.next[i], n>>7, e)
}

func genCases(g *Generator) {
	genCasesRec(g, 0, &topCase)
}

// i = byte position in byte slice b.
func genCasesRec(g *Generator, i int, c *Case) {
	if i == 0 {
		g.P("if len(b) == 0 { break }")
	} else {
		// TODO: premature eob error?
		g.P("if len(b) <= ", i, " { return proto.ErrInternalBadWireType }")
	}
	g.P("switch b[", i, "] {")
	g.In()
	var keys []int
	for k := range c.leaf {
		keys = append(keys, k)
	}
	for k := range c.next {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		g.P("case ", k, ":")
		g.In()
		if k < 128 {
			g.P("b = b[", i+1, ":]")
			genEntry(g, c.leaf[k])
		} else {
			genCasesRec(g, i+1, c.next[k])
		}
		g.Out()
	}
	g.Out()
	g.P("}")
}

func genEntry(g *Generator, e Entry) {
	field := e.field
	message := e.message
	fname := CamelCase(field.GetName())
	typeName := message.TypeName()
	ccTypeName := CamelCaseSlice(typeName)
	if field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REQUIRED {
		g.P("has", fname, " = true")
	}
	if e.packed {
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("if x > uint64(len(b)) { return proto.ErrInternalBadWireType }")
		g.P("p := b[:x]") // p = packed data
		g.P("b = b[x:]")

		g.P("for len(p) > 0 {")
		g.In()

		switch field.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
			g.P("if len(p) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := math.Float64frombits(uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 | uint64(p[3])<<24 | uint64(p[4])<<32 | uint64(p[5])<<40 | uint64(p[6])<<48 | uint64(p[7])<<56)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[8:]")
		case descriptor.FieldDescriptorProto_TYPE_FLOAT:
			g.P("if len(p) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := math.Float32frombits(uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 | uint32(p[3])<<24)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[4:]")
		case descriptor.FieldDescriptorProto_TYPE_INT64:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := int64(x)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_UINT64:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := uint64(x)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_SINT64:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := int64(x>>1) ^ int64(x)<<63>>63")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_INT32:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := int32(x)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_UINT32:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := uint32(x)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_SINT32:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := int32(x>>1) ^ int32(x)<<31>>31")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			g.P("x, n := proto.DecodeVarint(p)")
			g.P("if n == 0 { return proto.ErrInternalBadWireType }")
			g.P("p = p[n:]")
			g.P("v := ", g.TypeName(g.ObjectNamed(field.GetTypeName())), "(x)")
			g.P("m.", fname, "= append(m.", fname, ", v)")
		case descriptor.FieldDescriptorProto_TYPE_BOOL:
			g.P("v := false")
			g.P("if p[0] != 0 { v = true }")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[1:]")
		case descriptor.FieldDescriptorProto_TYPE_FIXED64:
			g.P("if len(p) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 | uint64(p[3])<<24 | uint64(p[4])<<32 | uint64(p[5])<<40 | uint64(p[6])<<48 | uint64(p[7])<<56")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[8:]")
		case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
			g.P("if len(p) < 8 { return proto.ErrInternalBadWireType }")
			g.P("v := int64(p[0]) | int64(p[1])<<8 | int64(p[2])<<16 | int64(p[3])<<24 | int64(p[4])<<32 | int64(p[5])<<40 | int64(p[6])<<48 | int64(p[7])<<56")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[8:]")
		case descriptor.FieldDescriptorProto_TYPE_FIXED32:
			g.P("if len(p) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 | uint32(p[3])<<24")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[4:]")
		case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
			g.P("if len(p) < 4 { return proto.ErrInternalBadWireType }")
			g.P("v := int32(p[0]) | int32(p[1])<<8 | int32(p[2])<<16 | int32(p[3])<<24")
			g.P("m.", fname, "= append(m.", fname, ", v)")
			g.P("p = p[4:]")
		}
		g.Out()
		g.P("}") // end of for len(p) > 0 loop
		g.P("continue loop")
		return
	}

	// Non-packed encoding.
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
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := int64(x)")
	case descriptor.FieldDescriptorProto_TYPE_UINT64:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := uint64(x)")
	case descriptor.FieldDescriptorProto_TYPE_SINT64:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := int64(x>>1) ^ int64(x)<<63>>63")
	case descriptor.FieldDescriptorProto_TYPE_INT32:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := int32(x)")
	case descriptor.FieldDescriptorProto_TYPE_UINT32:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := uint32(x)")
	case descriptor.FieldDescriptorProto_TYPE_SINT32:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("v := int32(x>>1) ^ int32(x)<<31>>31")
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		g.P("x, n := proto.DecodeVarint(b)")
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
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
		g.P("v := string(b[:x])")
		g.P("b = b[x:]")
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
		g.P("v := make([]byte, x)")
		g.P("copy(v, b)")
		g.P("b = b[x:]")
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		desc := g.ObjectNamed(field.GetTypeName())
		g.P("x, n := proto.DecodeVarint(b)")
		g.P("if n == 0 { return proto.ErrInternalBadWireType }")
		g.P("b = b[n:]")
		g.P("if uint64(len(b)) < x { return proto.ErrInternalBadWireType }")
		// TODO: merge if singleton, always add if repeated. Is that right?
		tname := g.TypeName(g.ObjectNamed(field.GetTypeName()))
		if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
			g.P("var v ", tname)
		} else if field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REPEATED {
			g.P("v := new(", tname, ")")
		} else {
			g.P("v := m.", fname)
			g.P("if v == nil {")
			g.In()
			g.P("v = new(", tname, ")")
			g.P("m.", fname, " = v")
			g.Out()
			g.P("}")
		}
		g.P("if err := v.MergeFullCustom(b[:x]); err != nil {")
		g.In()
		g.P("if r, ok := err.(*proto.RequiredNotSetError); ok {")
		g.In()
		g.P("r.AddParent(\"", field.GetName(), "\")")
		g.P("rnse = r")
		g.Out()
		g.P("} else {")
		g.In()
		g.P("return err")
		g.Out()
		g.P("}")
		g.Out()
		g.P("}")
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
		}
		if field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REPEATED {
			g.P("m.", fname, "= append(m.", fname, ", v)")
		}
		g.P("continue loop")
		return
	case descriptor.FieldDescriptorProto_TYPE_GROUP:
		g.P("i, j := proto.FindEndGroup(b)")
		g.P("if i < 0 { return proto.ErrInternalBadWireType }")
		g.P("var v ", g.TypeName(g.ObjectNamed(field.GetTypeName())))
		g.P("if err := v.MergeFullCustom(b[:i]); err != nil {")
		g.In()
		g.P("if r, ok := err.(*proto.RequiredNotSetError); ok {")
		g.In()
		g.P("r.AddParent(\"", field.GetName(), "\")")
		g.P("rnse = r")
		g.Out()
		g.P("} else {")
		g.In()
		g.P("return err")
		g.Out()
		g.P("}")
		g.Out()
		g.P("}")
		g.P("b = b[j:]")
	default:
		panic("unknown type for field: " + field.GetName())
	}

	// Value to write.
	v := "v"

	pointer := field.GetType() == descriptor.FieldDescriptorProto_TYPE_GROUP ||
		field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE

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
	default:
		g.P("m.", fname, "= ", v)
	}
	g.P("continue loop")
}

// Full custom decoder.
func (g *Generator) generateUnmarshalFullCustom(message *Descriptor) {
	typeName := message.TypeName()
	ccTypeName := CamelCaseSlice(typeName)

	// Build map from bytes on the wire to cases we need to handle.
	clearCases()
	for _, field := range message.Field {
		tag := int(field.GetNumber())
		var wire int
		couldBePacked := field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REPEATED
		switch field.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
			wire = wireFixed64
		case descriptor.FieldDescriptorProto_TYPE_FLOAT:
			wire = wireFixed32
		case descriptor.FieldDescriptorProto_TYPE_INT64:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_UINT64:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_SINT64:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_INT32:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_UINT32:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_SINT32:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_BOOL:
			wire = wireVarint
		case descriptor.FieldDescriptorProto_TYPE_FIXED64:
			wire = wireFixed64
		case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
			wire = wireFixed64
		case descriptor.FieldDescriptorProto_TYPE_FIXED32:
			wire = wireFixed32
		case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
			wire = wireFixed32
		case descriptor.FieldDescriptorProto_TYPE_STRING:
			wire = wireBytes
			couldBePacked = false
		case descriptor.FieldDescriptorProto_TYPE_BYTES:
			wire = wireBytes
			couldBePacked = false
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
			wire = wireBytes
			couldBePacked = false
		case descriptor.FieldDescriptorProto_TYPE_GROUP:
			wire = wireStartGroup
			couldBePacked = false
		default:
			panic("bad field type")
		}

		addCase(tag<<3+wire, Entry{field, message, false})
		if couldBePacked {
			addCase(tag<<3+wireBytes, Entry{field, message, true})
		}
	}

	// Parser header.
	g.P("func (m *", ccTypeName, ") MergeFullCustom(b []byte) error {")
	g.In()
	for _, field := range message.Field {
		if field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REQUIRED {
			fname := CamelCase(field.GetName())
			g.P("has", fname, " := false")
		}
	}
	for _, field := range message.Field {
		if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE ||
			field.GetType() == descriptor.FieldDescriptorProto_TYPE_GROUP {
			g.P("var rnse *proto.RequiredNotSetError")
			break
		}
	}

	// Main parsing loop.
	g.P("loop: for {")
	g.In()
	genCases(g)
	// TODO: we get here if we know the tag but the wiretype is wrong.  What to do?
	// Some sort of slow path here (pass known tags?).
	if message.proto3() {
		g.P("b = proto.SkipUnrecognized2(b, nil)")
	} else {
		g.P("b = proto.SkipUnrecognized2(b, &m.XXX_unrecognized)")
	}
	g.Out()
	g.P("}")

	// Post-process & return.
	for _, field := range message.Field {
		if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE ||
			field.GetType() == descriptor.FieldDescriptorProto_TYPE_GROUP {
			// If a submessage is missing a required field, report that first.
			// (TODO: Does it really matter why? Some tests check for this.)
			g.P("if rnse != nil { return rnse }")
			break
		}
	}
	for _, field := range message.Field {
		if field.GetLabel() == descriptor.FieldDescriptorProto_LABEL_REQUIRED {
			fname := CamelCase(field.GetName())
			g.P("if !has", fname, "{ return &proto.RequiredNotSetError{\"", field.GetName(), "\"} }")
		}
	}
	g.P("return nil")
	g.Out()
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
