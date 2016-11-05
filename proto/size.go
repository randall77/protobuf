// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2010 The Go Authors.  All rights reserved.
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

package proto

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

type sizeInfo struct {
	typ reflect.Type // type of the pb struct

	// 0 = only typ field is initialized
	// 1 = completely initialized
	initialized int32
	lock        sync.Mutex // prevents double initialization

	fields []sizeFieldInfo // one record for each field to be encoded
}

// A sizer2 takes a pointer to a message field and returns the number of bytes
// it will take to encode.
type sizer2 func(f unsafe.Pointer) int

type sizeFieldInfo struct {
	// offset of the field in the proto message structure.
	offset uintptr
	// function used to compute its encoded size.
	size sizer2

	// TODO: make this generally an encoding field info structure.
	// It will have a sizer and an encoder.
}

func SizeIt(pb Message) int {
	var b Buffer
	return b.SizeIt(pb)
}

func (b *Buffer) SizeIt(pb Message) int {
	if pb == b.lastEncMsg {
		return b.lastSizeInfo.size(b.lastEncPtr)
	}
	v := reflect.ValueOf(pb)
	t := v.Type().Elem()
	s := getSizeInfo(t)
	m := unsafe.Pointer(v.Pointer())
	b.lastEncMsg = pb
	b.lastEncPtr = m
	b.lastSizeInfo = s
	return s.size(m)
}

// size is the main function which computes the size of a message m.
// s contains type information for m.
func (s *sizeInfo) size(m unsafe.Pointer) int {
	if atomic.LoadInt32(&s.initialized) == 0 {
		s.computeSizeInfo()
	}
	n := 0
	for _, f := range s.fields {
		n += f.size(unsafe.Pointer(uintptr(m) + f.offset))
	}
	return n
}

// getSizeInfo returns the data structure which can be
// subsequently used to size a message of the given type.
// t is the type of the message (note: not pointer to message).
func getSizeInfo(t reflect.Type) *sizeInfo {
	// It would be correct to return a new sizeInfo
	// unconditionally. We would end up allocating one
	// per occurrence of that type as a message or submessage.
	// We use a cache here just to reduce memory usage.
	sizeLock.Lock()
	defer sizeLock.Unlock()
	s := sizeMap[t]
	if s == nil {
		s = &sizeInfo{typ: t}
		// Note: we just set the type here. The rest of the fields
		// will be initialized on first use.
		sizeMap[t] = s
	}
	return s
}

var sizeLock sync.Mutex
var sizeMap = map[reflect.Type]*sizeInfo{}

// computeSizeInfo fills in u with information for use
// in sizing protocol buffers of type u.typ.
func (s *sizeInfo) computeSizeInfo() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.initialized != 0 {
		return
	}
	t := s.typ

	// List of the generated type and offset for each oneof field.
	type oneofField struct {
		ityp   reflect.Type // interface type of oneof field
		offset uintptr      // offset in containing message
	}
	var oneofFields []oneofField
	_ = oneofFields

	n := t.NumField()
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.Name == "XXX_unrecognized" {
			// The byte slice used to hold unrecognized input is special.
			s.fields = append(s.fields, sizeFieldInfo{f.Offset, unrecognizedSizer})
			continue
		}
		if f.Name == "XXX_InternalExtensions" {
			s.fields = append(s.fields, sizeFieldInfo{f.Offset, extensionSizer})
			continue
		}

		oneof := f.Tag.Get("protobuf_oneof")
		if oneof != "" {
			//oneofFields = append(oneofFields, oneofField{f.Type, f.Offset})
			// The rest of oneof processing happens below.
			//continue
		}

		// Extract sizing function from the field (its type and tags).
		sizer := fieldSizer(&f)
		s.fields = append(s.fields, sizeFieldInfo{f.Offset, sizer})
	}

	// Find any types associated with oneof fields.
	// TODO: XXX_OneofFuncs returns more info than we need.  Get rid of some of it?
	/*
		fn := reflect.Zero(reflect.PtrTo(t)).MethodByName("XXX_OneofFuncs")
		if fn.IsValid() {
			res := fn.Call(nil)[3] // last return value from XXX_OneofFuncs: []interface{}
			for i := res.Len() - 1; i >= 0; i-- {
				v := res.Index(i)                             // interface{}
				tptr := reflect.ValueOf(v.Interface()).Type() // *Msg_X
				typ := tptr.Elem()                            // Msg_X

				f := typ.Field(0) // oneof implementers have one field
				baseSize := fieldSizer(&f)
				tag, err := strconv.Atoi(strings.Split(f.Tag.Get("protobuf"), ",")[1])
				if err != nil {
					panic("protobuf tag field not an integer")
				}

				// Find the oneof field that this struct implements.
				// Might take O(n^2) to process all of the oneofs, but who cares.
				for _, of := range oneofFields {
					if tptr.Implements(of.ityp) {
						// We have found the corresponding interface for this struct.
						// That lets us know where this struct should be stored
						// when we encounter it during sizeing.
						size := makeSizeOneof(typ, of.ityp, baseSize)
						u.setTag(tag, n, of.offset, size, false)
					}
				}
			}
		}
	*/

	atomic.StoreInt32(&s.initialized, 1)
}

func fieldSizer(f *reflect.StructField) sizer2 {
	return typeSizer(f.Type, f.Tag.Get("protobuf"), f)
}

func typeSizer(t reflect.Type, tags string, f *reflect.StructField) sizer2 {
	tagArray := strings.Split(tags, ",")
	encoding := tagArray[0]
	tag, err := strconv.Atoi(tagArray[1])
	if err != nil {
		panic("protobuf tag field not an integer")
	}
	_ = tagArray[2] // kind (rep,opt,req)

	tagSize := sizeVarint(uint64(tag) << 3) // note: same size regardless of wiretype

	// Figure out packaging (pointer, slice, or both)
	slice := false
	pointer := false
	if t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8 {
		slice = true
		t = t.Elem()
	}
	if t.Kind() == reflect.Ptr {
		pointer = true
		t = t.Elem()
	}

	// We'll never have both pointer and slice for basic types.
	if pointer && slice && t.Kind() != reflect.Struct {
		panic("both pointer and slice for basic type")
	}

	packed := len(tagArray) >= 4 && tagArray[3] == "packed"

	switch t.Kind() {
	case reflect.Bool:
		if pointer {
			return makeSizeBoolPtr(tagSize)
		}
		if packed {
			return makeSizeBoolPacked(tagSize)
		}
		if slice {
			return makeSizeBoolSlice(tagSize)
		}
		return makeSizeBoolValue(tagSize)

	case reflect.Int32:
		switch encoding {
		case "varint":
			// this could be int32 or enum
			if pointer {
				return makeSizeInt32Ptr(tagSize)
			}
			if packed {
				return makeSizeInt32Packed(tagSize)
			}
			if slice {
				return makeSizeInt32Slice(tagSize)
			}
			return makeSizeInt32Value(tagSize)
		case "zigzag32":
			if pointer {
				return makeSizeSint32Ptr(tagSize)
			}
			if packed {
				return makeSizeSint32Packed(tagSize)
			}
			if slice {
				return makeSizeSint32Slice(tagSize)
			}
			return makeSizeSint32Value(tagSize)
		}
	case reflect.Int64:
		switch encoding {
		case "varint":
			if pointer {
				return makeSizeInt64Ptr(tagSize)
			}
			if packed {
				return makeSizeInt64Packed(tagSize)
			}
			if slice {
				return makeSizeInt64Slice(tagSize)
			}
			return makeSizeInt64Value(tagSize)
		case "zigzag64":
			if pointer {
				return makeSizeSint64Ptr(tagSize)
			}
			if packed {
				return makeSizeSint64Packed(tagSize)
			}
			if slice {
				return makeSizeSint64Slice(tagSize)
			}
			return makeSizeSint64Value(tagSize)
		}
	case reflect.Uint32:
		switch encoding {
		case "varint":
			if pointer {
				return makeSizeInt32Ptr(tagSize)
			}
			if packed {
				return makeSizeInt32Packed(tagSize)
			}
			if slice {
				return makeSizeInt32Slice(tagSize)
			}
			return makeSizeInt32Value(tagSize)
		case "fixed32":
			if pointer {
				return makeSizeFixed32Ptr(tagSize)
			}
			if packed {
				return makeSizeFixed32Packed(tagSize)
			}
			if slice {
				return makeSizeFixed32Slice(tagSize)
			}
			return makeSizeFixed32Value(tagSize)
		}
	case reflect.Uint64:
		switch encoding {
		case "varint":
			if pointer {
				return makeSizeInt64Ptr(tagSize)
			}
			if packed {
				return makeSizeInt64Packed(tagSize)
			}
			if slice {
				return makeSizeInt64Slice(tagSize)
			}
			return makeSizeInt64Value(tagSize)
		case "fixed64":
			if pointer {
				return makeSizeFixed64Ptr(tagSize)
			}
			if packed {
				return makeSizeFixed64Packed(tagSize)
			}
			if slice {
				return makeSizeFixed64Slice(tagSize)
			}
			return makeSizeFixed64Value(tagSize)
		}
	case reflect.Float32:
		if pointer {
			return makeSizeFixed32Ptr(tagSize)
		}
		if packed {
			return makeSizeFixed32Packed(tagSize)
		}
		if slice {
			return makeSizeFixed32Slice(tagSize)
		}
		return makeSizeFixed32Value(tagSize)
	case reflect.Float64:
		if pointer {
			return makeSizeFixed64Ptr(tagSize)
		}
		if packed {
			return makeSizeFixed64Packed(tagSize)
		}
		if slice {
			return makeSizeFixed64Slice(tagSize)
		}
		return makeSizeFixed64Value(tagSize)
	case reflect.Slice:
		if slice {
			return makeSizeBytesSlice(tagSize)
		}
		return makeSizeBytesValue(tagSize)
	case reflect.String:
		if pointer {
			return makeSizeStringPtr(tagSize)
		}
		if slice {
			return makeSizeStringSlice(tagSize)
		}
		return makeSizeStringValue(tagSize)
	case reflect.Struct:
		// message or group field
		if !pointer {
			panic(fmt.Sprintf("message/group field %s:%s without pointer", t, encoding))
		}
		switch encoding {
		case "bytes":
			if slice {
				return makeSizeMessageSlicePtr(tagSize, getSizeInfo(t))
			}
			return makeSizeMessagePtr(tagSize, getSizeInfo(t))
		case "group":
			if slice {
				return makeSizeGroupSlicePtr(tagSize, getSizeInfo(t))
			}
			return makeSizeGroupPtr(tagSize, getSizeInfo(t))
		}
	case reflect.Map:
		return makeSizeMap(tagSize, t, f)
	case reflect.Interface:
		return makeSizeOneof(f.Type)
	}
	panic(fmt.Sprintf("sizer not found type:%s encoding:%s", t, encoding))

}

func makeSizeFloat64Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*float64)(f)
		if v == 0.0 { // TODO: use int64 for this test? What are the encoding semantics of -0.0?
			return 0
		}
		return tagSize + 8
	}
}

func makeSizeInt64Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int64)(f)
		if p == nil {
			return 0
		}
		v := *p
		return tagSize + sizeVarint(uint64(v))
	}
}

func makeSizeInt64Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		if len(s) == 0 {
			return 0
		}
		n := 0
		for _, v := range s {
			n += sizeVarint(uint64(v))
		}
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeInt64Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		n := 0
		for _, v := range s {
			n += tagSize + sizeVarint(uint64(v))
		}
		return n
	}
}

func makeSizeInt64Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int64)(f)
		if v == 0 {
			return 0
		}
		return tagSize + sizeVarint(uint64(v))
	}
}

func makeSizeSint64Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int64)(f)
		if p == nil {
			return 0
		}
		v := *p
		return tagSize + sizeZigzag64(uint64(v))
	}
}

func makeSizeSint64Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		if len(s) == 0 {
			return 0
		}
		n := 0
		for _, v := range s {
			n += sizeZigzag64(uint64(v))
		}
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeSint64Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		n := 0
		for _, v := range s {
			n += tagSize + sizeZigzag64(uint64(v))
		}
		return n
	}
}

func makeSizeSint64Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int64)(f)
		if v == 0 {
			return 0
		}
		return tagSize + sizeZigzag64(uint64(v))
	}
}

func makeSizeInt32Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int32)(f)
		if p == nil {
			return 0
		}
		v := *p
		return tagSize + sizeVarint(uint64(v))
	}
}

func makeSizeInt32Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		if len(s) == 0 {
			return 0
		}
		n := 0
		for _, v := range s {
			n += sizeVarint(uint64(v))
		}
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeInt32Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		n := 0
		for _, v := range s {
			n += tagSize + sizeVarint(uint64(v))
		}
		return n
	}
}

func makeSizeInt32Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int32)(f)
		if v == 0 {
			return 0
		}
		return tagSize + sizeVarint(uint64(v))
	}
}

func makeSizeSint32Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int32)(f)
		if p == nil {
			return 0
		}
		v := *p
		return tagSize + sizeZigzag32(uint64(v)) // TODO: check this cast (+3 below)
	}
}

func makeSizeSint32Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		if len(s) == 0 {
			return 0
		}
		n := 0
		for _, v := range s {
			n += sizeZigzag32(uint64(v))
		}
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeSint32Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		n := 0
		for _, v := range s {
			n += tagSize + sizeZigzag32(uint64(v))
		}
		return n
	}
}

func makeSizeSint32Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int32)(f)
		if v == 0 {
			return 0
		}
		return tagSize + sizeZigzag32(uint64(v))
	}
}

func makeSizeBoolPtr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**bool)(f)
		if p == nil {
			return 0
		}
		return tagSize + 1
	}
}

func makeSizeBoolPacked(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]bool)(f)
		n := len(s)
		if n == 0 {
			return 0
		}
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeBoolSlice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]bool)(f)
		return len(s) * (tagSize + 1)
	}
}

func makeSizeBoolValue(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*bool)(f)
		if !v {
			return 0
		}
		return tagSize + 1
	}
}

func makeSizeFixed32Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int32)(f)
		if p == nil {
			return 0
		}
		return tagSize + 4
	}
}

func makeSizeFixed32Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		n := len(s)
		if n == 0 {
			return 0
		}
		n *= 4
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeFixed32Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int32)(f)
		return len(s) * (tagSize + 4)
	}
}

func makeSizeFixed32Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int32)(f)
		if v == 0 { // Note: float32,int32,uint32 all have the same zero encoding.
			return 0
		}
		return tagSize + 4
	}
}

func makeSizeFixed64Ptr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**int64)(f)
		if p == nil {
			return 0
		}
		return tagSize + 8
	}
}

func makeSizeFixed64Packed(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		n := len(s)
		if n == 0 {
			return 0
		}
		n *= 8
		n += sizeVarint(uint64(n))
		n += tagSize
		return n
	}
}

func makeSizeFixed64Slice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]int64)(f)
		return len(s) * (tagSize + 8)
	}
}

func makeSizeFixed64Value(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*int64)(f)
		if v == 0 { // Note: float64,int64,uint64 all have the same zero encoding.
			return 0
		}
		return tagSize + 8
	}
}

func makeSizeBytesSlice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[][]byte)(f)
		n := 0
		for _, b := range s {
			n += tagSize + sizeVarint(uint64(len(b))) + len(b)
		}
		return n
	}
}

func makeSizeBytesValue(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		b := *(*[]byte)(f)
		if len(b) == 0 {
			return 0
		}
		return tagSize + sizeVarint(uint64(len(b))) + len(b)
	}
}

func makeSizeStringPtr(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		p := *(**string)(f)
		if p == nil {
			return 0
		}
		v := *p
		return tagSize + sizeVarint(uint64(len(v))) + len(v)
	}
}

func makeSizeStringSlice(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		s := *(*[]string)(f)
		n := 0
		for _, v := range s {
			n += tagSize + sizeVarint(uint64(len(v))) + len(v)
		}
		return n
	}
}

func makeSizeStringValue(tagSize int) sizer2 {
	return func(f unsafe.Pointer) int {
		v := *(*string)(f)
		if v == "" {
			return 0
		}
		return tagSize + sizeVarint(uint64(len(v))) + len(v)
	}
}

func makeSizeMessagePtr(tagSize int, s *sizeInfo) sizer2 {
	return func(f unsafe.Pointer) int {
		m := *(*unsafe.Pointer)(f)
		if m == nil {
			return 0
		}
		n := s.size(m)
		return tagSize + sizeVarint(uint64(n)) + n
	}
}

func makeSizeMessageSlicePtr(tagSize int, s *sizeInfo) sizer2 {
	return func(f unsafe.Pointer) int {
		a := *(*[]unsafe.Pointer)(f)
		n := 0
		for _, m := range a {
			nn := s.size(m)
			n += tagSize + sizeVarint(uint64(nn)) + nn
		}
		return n
	}
}

func makeSizeGroupPtr(tagSize int, s *sizeInfo) sizer2 {
	tagSize *= 2 // both begin and end tag
	return func(f unsafe.Pointer) int {
		m := *(*unsafe.Pointer)(f)
		if m == nil {
			return 0
		}
		return tagSize + s.size(m)
	}
}

func makeSizeGroupSlicePtr(tagSize int, s *sizeInfo) sizer2 {
	tagSize *= 2 // both begin and end tag
	return func(f unsafe.Pointer) int {
		a := *(*[]unsafe.Pointer)(f)
		n := 0
		for _, m := range a {
			n += tagSize + s.size(m)
		}
		return n
	}
}

func makeSizeMap(tagSize int, t reflect.Type, f *reflect.StructField) sizer2 {
	kt := t.Key()
	vt := t.Elem()
	keySizer := typeSizer(kt, f.Tag.Get("protobuf_key"), nil)
	valSizer := typeSizer(vt, f.Tag.Get("protobuf_val"), nil)
	kstore := reflect.New(kt)
	vstore := reflect.New(vt)
	return func(f unsafe.Pointer) int {
		m := reflect.NewAt(t, f).Elem()
		n := 0
		for _, k := range m.MapKeys() {
			v := m.MapIndex(k)
			// copy k,v to addressable memory
			kstore.Elem().Set(k)
			vstore.Elem().Set(v)
			// compute size of {key,value} message
			n += tagSize
			n += 1 // key tag is 1
			n += keySizer(unsafe.Pointer(kstore.Pointer()))
			n += 1 // val tag is 2
			n += valSizer(unsafe.Pointer(vstore.Pointer()))
		}
		return n
	}
}

func makeSizeOneof(t reflect.Type) sizer2 {
	// TODO
	return nil
}

func unrecognizedSizer(f unsafe.Pointer) int {
	// Unrecognized bytes are copied verbatim.
	return len(*(*[]byte)(f))
}
func extensionSizer(f unsafe.Pointer) int {
	// TODO
	return 0
}
