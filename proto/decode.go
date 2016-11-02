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

/*
 * Routines for decoding protocol buffer data to construct in-memory representations.
 */

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// errOverflow is returned when an integer is too large to be represented.
var errOverflow = errors.New("proto: integer overflow")

// ErrInternalBadWireType is returned by generated code when an incorrect
// wire type is encountered. It does not get returned to user code.
var ErrInternalBadWireType = errors.New("proto: internal error: bad wiretype for oneof")

// Error used by decode internally.
var errInternalBadWireType = errors.New("proto: internal error: bad wiretype for oneof")

// The fundamental decoders that interpret bytes on the wire.
// Those that take integer types all return uint64 and are
// therefore of type valueDecoder.

// DecodeVarint reads a varint-encoded integer from the slice.
// It returns the integer and the number of bytes consumed, or
// zero if there is not enough.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func DecodeVarint(buf []byte) (uint64, int) {
	if len(buf) == 0 {
		return 0, 0
	} else if buf[0] < 0x80 {
		return uint64(buf[0]), 1
	} else if len(buf) < 10 {
		// near end of buffer; tread carefully.
		x := uint64(buf[0]) & 0x7f
		n := 1
		for shift := uint(7); shift < 64; shift += 7 {
			if n >= len(buf) {
				return 0, 0
			}
			b := uint64(buf[n])
			n++
			x |= (b & 0x7F) << shift
			if (b & 0x80) == 0 {
				return x, n
			}
		}
	}

	// we already checked the first byte
	x := uint64(buf[0]) - 0x80

	b := uint64(buf[1])
	x += b << 7
	if b&0x80 == 0 {
		return x, 2
	}
	x -= 0x80 << 7

	b = uint64(buf[2])
	x += b << 14
	if b&0x80 == 0 {
		return x, 3
	}
	x -= 0x80 << 14

	b = uint64(buf[3])
	x += b << 21
	if b&0x80 == 0 {
		return x, 4
	}
	x -= 0x80 << 21

	b = uint64(buf[4])
	x += b << 28
	if b&0x80 == 0 {
		return x, 5
	}
	x -= 0x80 << 28

	b = uint64(buf[5])
	x += b << 35
	if b&0x80 == 0 {
		return x, 6
	}
	x -= 0x80 << 35

	b = uint64(buf[6])
	x += b << 42
	if b&0x80 == 0 {
		return x, 7
	}
	x -= 0x80 << 42

	b = uint64(buf[7])
	x += b << 49
	if b&0x80 == 0 {
		return x, 8
	}
	x -= 0x80 << 49

	b = uint64(buf[8])
	x += b << 56
	if b&0x80 == 0 {
		return x, 9
	}
	x -= 0x80 << 56

	b = uint64(buf[9])
	x += b << 63
	if b&0x80 == 0 {
		return x, 10
	}
	// x -= 0x80 << 63 // Always zero.

	return 0, 0
}

func (p *Buffer) decodeVarintSlow() (x uint64, err error) {
	i := p.index
	l := len(p.buf)

	for shift := uint(0); shift < 64; shift += 7 {
		if i >= l {
			err = io.ErrUnexpectedEOF
			return
		}
		b := p.buf[i]
		i++
		x |= (uint64(b) & 0x7F) << shift
		if b < 0x80 {
			p.index = i
			return
		}
	}

	// The number is too large to represent in a 64-bit value.
	err = errOverflow
	return
}

// DecodeVarint reads a varint-encoded integer from the Buffer.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func (p *Buffer) DecodeVarint() (x uint64, err error) {
	i := p.index
	buf := p.buf

	if i >= len(buf) {
		return 0, io.ErrUnexpectedEOF
	} else if buf[i] < 0x80 {
		p.index++
		return uint64(buf[i]), nil
	} else if len(buf)-i < 10 {
		return p.decodeVarintSlow()
	}

	var b uint64
	// we already checked the first byte
	x = uint64(buf[i]) - 0x80
	i++

	b = uint64(buf[i])
	i++
	x += b << 7
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 7

	b = uint64(buf[i])
	i++
	x += b << 14
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 14

	b = uint64(buf[i])
	i++
	x += b << 21
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 21

	b = uint64(buf[i])
	i++
	x += b << 28
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 28

	b = uint64(buf[i])
	i++
	x += b << 35
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 35

	b = uint64(buf[i])
	i++
	x += b << 42
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 42

	b = uint64(buf[i])
	i++
	x += b << 49
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 49

	b = uint64(buf[i])
	i++
	x += b << 56
	if b&0x80 == 0 {
		goto done
	}
	x -= 0x80 << 56

	b = uint64(buf[i])
	i++
	x += b << 63
	if b&0x80 == 0 {
		goto done
	}
	// x -= 0x80 << 63 // Always zero.

	return 0, errOverflow

done:
	p.index = i
	return x, nil
}

// DecodeFixed64 reads a 64-bit integer from the Buffer.
// This is the format for the
// fixed64, sfixed64, and double protocol buffer types.
func (p *Buffer) DecodeFixed64() (x uint64, err error) {
	// x, err already 0
	i := p.index + 8
	if i < 0 || i > len(p.buf) {
		err = io.ErrUnexpectedEOF
		return
	}
	p.index = i

	x = uint64(p.buf[i-8])
	x |= uint64(p.buf[i-7]) << 8
	x |= uint64(p.buf[i-6]) << 16
	x |= uint64(p.buf[i-5]) << 24
	x |= uint64(p.buf[i-4]) << 32
	x |= uint64(p.buf[i-3]) << 40
	x |= uint64(p.buf[i-2]) << 48
	x |= uint64(p.buf[i-1]) << 56
	return
}

// DecodeFixed32 reads a 32-bit integer from the Buffer.
// This is the format for the
// fixed32, sfixed32, and float protocol buffer types.
func (p *Buffer) DecodeFixed32() (x uint64, err error) {
	// x, err already 0
	i := p.index + 4
	if i < 0 || i > len(p.buf) {
		err = io.ErrUnexpectedEOF
		return
	}
	p.index = i

	x = uint64(p.buf[i-4])
	x |= uint64(p.buf[i-3]) << 8
	x |= uint64(p.buf[i-2]) << 16
	x |= uint64(p.buf[i-1]) << 24
	return
}

// DecodeZigzag64 reads a zigzag-encoded 64-bit integer
// from the Buffer.
// This is the format used for the sint64 protocol buffer type.
func (p *Buffer) DecodeZigzag64() (x uint64, err error) {
	x, err = p.DecodeVarint()
	if err != nil {
		return
	}
	x = (x >> 1) ^ uint64((int64(x&1)<<63)>>63)
	return
}

// DecodeZigzag32 reads a zigzag-encoded 32-bit integer
// from  the Buffer.
// This is the format used for the sint32 protocol buffer type.
func (p *Buffer) DecodeZigzag32() (x uint64, err error) {
	x, err = p.DecodeVarint()
	if err != nil {
		return
	}
	x = uint64((uint32(x) >> 1) ^ uint32((int32(x&1)<<31)>>31))
	return
}

// These are not ValueDecoders: they produce an array of bytes or a string.
// bytes, embedded messages

// DecodeRawBytes reads a count-delimited byte buffer from the Buffer.
// This is the format used for the bytes protocol buffer
// type and for embedded messages.
func (p *Buffer) DecodeRawBytes(alloc bool) (buf []byte, err error) {
	n, err := p.DecodeVarint()
	if err != nil {
		return nil, err
	}

	nb := int(n)
	if nb < 0 {
		return nil, fmt.Errorf("proto: bad byte length %d", nb)
	}
	end := p.index + nb
	if end < p.index || end > len(p.buf) {
		return nil, io.ErrUnexpectedEOF
	}

	if !alloc {
		// todo: check if can get more uses of alloc=false
		buf = p.buf[p.index:end]
		p.index += nb
		return
	}

	buf = make([]byte, nb)
	copy(buf, p.buf[p.index:])
	p.index += nb
	return
}

// DecodeStringBytes reads an encoded string from the Buffer.
// This is the format used for the proto2 string type.
func (p *Buffer) DecodeStringBytes() (s string, err error) {
	buf, err := p.DecodeRawBytes(false)
	if err != nil {
		return
	}
	return string(buf), nil
}

// Skip the next item in the buffer. Its wire type is decoded and presented as an argument.
// If the protocol buffer has extensions, and the field matches, add it as an extension.
// Otherwise, if the XXX_unrecognized field exists, append the skipped data there.
func (o *Buffer) skipAndSave(t reflect.Type, tag, wire int, base structPointer, unrecField field) error {
	oi := o.index

	err := o.skip(t, tag, wire)
	if err != nil {
		return err
	}

	if !unrecField.IsValid() {
		return nil
	}

	ptr := structPointer_Bytes(base, unrecField)

	// Add the skipped field to struct field
	obuf := o.buf

	o.buf = *ptr
	o.EncodeVarint(uint64(tag<<3 | wire))
	*ptr = append(o.buf, obuf[oi:o.index]...)

	o.buf = obuf

	return nil
}

// Skip the next item in the buffer. Its wire type is decoded and presented as an argument.
func (o *Buffer) skip(t reflect.Type, tag, wire int) error {

	var u uint64
	var err error

	switch wire {
	case WireVarint:
		_, err = o.DecodeVarint()
	case WireFixed64:
		_, err = o.DecodeFixed64()
	case WireBytes:
		_, err = o.DecodeRawBytes(false)
	case WireFixed32:
		_, err = o.DecodeFixed32()
	case WireStartGroup:
		for {
			u, err = o.DecodeVarint()
			if err != nil {
				break
			}
			fwire := int(u & 0x7)
			if fwire == WireEndGroup {
				break
			}
			ftag := int(u >> 3)
			err = o.skip(t, ftag, fwire)
			if err != nil {
				break
			}
		}
	default:
		err = fmt.Errorf("proto: can't skip unknown wire type %d for %s", wire, t)
	}
	return err
}

// Unmarshaler is the interface representing objects that can
// unmarshal themselves.  The method should reset the receiver before
// decoding starts.  The argument points to data that may be
// overwritten, so implementations should not keep references to the
// buffer.
type Unmarshaler interface {
	Unmarshal([]byte) error
}

// Unmarshal parses the protocol buffer representation in buf and places the
// decoded result in pb.  If the struct underlying pb does not match
// the data in buf, the results can be unpredictable.
//
// Unmarshal resets pb before starting to unmarshal, so any
// existing data in pb is always removed. Use UnmarshalMerge
// to preserve and append to existing data.
func Unmarshal2(buf []byte, pb Message) error {
	pb.Reset()
	return UnmarshalMerge(buf, pb)
}

// UnmarshalMerge parses the protocol buffer representation in buf and
// writes the decoded result to pb.  If the struct underlying pb does not match
// the data in buf, the results can be unpredictable.
//
// UnmarshalMerge merges into existing data in pb.
// Most code should use Unmarshal instead.
func UnmarshalMerge2(buf []byte, pb Message) error {
	// If the object can unmarshal itself, let it.
	if u, ok := pb.(Unmarshaler); ok {
		return u.Unmarshal(buf)
	}
	return NewBuffer(buf).Unmarshal(pb)
}

// DecodeMessage reads a count-delimited message from the Buffer.
func (p *Buffer) DecodeMessage2(pb Message) error {
	enc, err := p.DecodeRawBytes(false)
	if err != nil {
		return err
	}
	return NewBuffer(enc).Unmarshal(pb)
}

// DecodeGroup reads a tag-delimited group from the Buffer.
func (p *Buffer) DecodeGroup2(pb Message) error {
	typ, base, err := getbase(pb)
	if err != nil {
		return err
	}
	return p.unmarshalType(typ.Elem(), GetProperties(typ.Elem()), true, base)
}

// Unmarshal parses the protocol buffer representation in the
// Buffer and places the decoded result in pb.  If the struct
// underlying pb does not match the data in the buffer, the results can be
// unpredictable.
//
// Unlike proto.Unmarshal, this does not reset pb before starting to unmarshal.
func (p *Buffer) Unmarshal2(pb Message) error {
	// If the object can unmarshal itself, let it.
	if u, ok := pb.(Unmarshaler); ok {
		err := u.Unmarshal(p.buf[p.index:])
		p.index = len(p.buf)
		return err
	}

	typ, base, err := getbase(pb)
	if err != nil {
		return err
	}

	err = p.unmarshalType(typ.Elem(), GetProperties(typ.Elem()), false, base)

	if collectStats {
		stats.Decode++
	}

	return err
}

// unmarshalType does the work of unmarshaling a structure.
func (o *Buffer) unmarshalType(st reflect.Type, prop *StructProperties, is_group bool, base structPointer) error {
	var state errorState
	required, reqFields := prop.reqCount, uint64(0)

	var err error
	for err == nil && o.index < len(o.buf) {
		oi := o.index
		var u uint64
		u, err = o.DecodeVarint()
		if err != nil {
			break
		}
		wire := int(u & 0x7)
		if wire == WireEndGroup {
			if is_group {
				if required > 0 {
					// Not enough information to determine the exact field.
					// (See below.)
					return &RequiredNotSetError{"{Unknown}"}
				}
				return nil // input is satisfied
			}
			return fmt.Errorf("proto: %s: wiretype end group for non-group", st)
		}
		tag := int(u >> 3)
		if tag <= 0 {
			return fmt.Errorf("proto: %s: illegal tag %d (wire type %d)", st, tag, wire)
		}
		fieldnum, ok := prop.decoderTags.get(tag)
		if !ok {
			// Maybe it's an extension?
			if prop.extendable {
				if e, _ := extendable(structPointer_Interface(base, st)); isExtensionField(e, int32(tag)) {
					if err = o.skip(st, tag, wire); err == nil {
						extmap := e.extensionsWrite()
						ext := extmap[int32(tag)] // may be missing
						ext.enc = append(ext.enc, o.buf[oi:o.index]...)
						extmap[int32(tag)] = ext
					}
					continue
				}
			}
			// Maybe it's a oneof?
			if prop.oneofUnmarshaler != nil {
				m := structPointer_Interface(base, st).(Message)
				// First return value indicates whether tag is a oneof field.
				ok, err = prop.oneofUnmarshaler(m, tag, wire, o)
				if err == ErrInternalBadWireType {
					// Map the error to something more descriptive.
					// Do the formatting here to save generated code space.
					err = fmt.Errorf("bad wiretype for oneof field in %T", m)
				}
				if ok {
					continue
				}
			}
			err = o.skipAndSave(st, tag, wire, base, prop.unrecField)
			continue
		}
		p := prop.Prop[fieldnum]

		if p.dec == nil {
			fmt.Fprintf(os.Stderr, "proto: no protobuf decoder for %s.%s\n", st, st.Field(fieldnum).Name)
			continue
		}
		dec := p.dec
		if wire != WireStartGroup && wire != p.WireType {
			if wire == WireBytes && p.packedDec != nil {
				// a packable field
				dec = p.packedDec
			} else {
				err = fmt.Errorf("proto: bad wiretype for field %s.%s: got wiretype %d, want %d", st, st.Field(fieldnum).Name, wire, p.WireType)
				continue
			}
		}
		decErr := dec(o, p, base)
		if decErr != nil && !state.shouldContinue(decErr, p) {
			err = decErr
		}
		if err == nil && p.Required {
			// Successfully decoded a required field.
			if tag <= 64 {
				// use bitmap for fields 1-64 to catch field reuse.
				var mask uint64 = 1 << uint64(tag-1)
				if reqFields&mask == 0 {
					// new required field
					reqFields |= mask
					required--
				}
			} else {
				// This is imprecise. It can be fooled by a required field
				// with a tag > 64 that is encoded twice; that's very rare.
				// A fully correct implementation would require allocating
				// a data structure, which we would like to avoid.
				required--
			}
		}
	}
	if err == nil {
		if is_group {
			return io.ErrUnexpectedEOF
		}
		if state.err != nil {
			return state.err
		}
		if required > 0 {
			// Not enough information to determine the exact field. If we use extra
			// CPU, we could determine the field only if the missing required field
			// has a tag <= 64 and we check reqFields.
			return &RequiredNotSetError{"{Unknown}"}
		}
	}
	return err
}

// Individual type decoders
// For each,
//	u is the decoded value,
//	v is a pointer to the field (pointer) in the struct

// Sizes of the pools to allocate inside the Buffer.
// The goal is modest amortization and allocation
// on at least 16-byte boundaries.
const (
	boolPoolSize   = 16
	uint32PoolSize = 8
	uint64PoolSize = 4
)

// Decode a bool.
func (o *Buffer) dec_bool(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	if len(o.bools) == 0 {
		o.bools = make([]bool, boolPoolSize)
	}
	o.bools[0] = u != 0
	*structPointer_Bool(base, p.field) = &o.bools[0]
	o.bools = o.bools[1:]
	return nil
}

func (o *Buffer) dec_proto3_bool(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	*structPointer_BoolVal(base, p.field) = u != 0
	return nil
}

// Decode an int32.
func (o *Buffer) dec_int32(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	word32_Set(structPointer_Word32(base, p.field), o, uint32(u))
	return nil
}

func (o *Buffer) dec_proto3_int32(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	word32Val_Set(structPointer_Word32Val(base, p.field), uint32(u))
	return nil
}

// Decode an int64.
func (o *Buffer) dec_int64(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	word64_Set(structPointer_Word64(base, p.field), o, u)
	return nil
}

func (o *Buffer) dec_proto3_int64(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	word64Val_Set(structPointer_Word64Val(base, p.field), o, u)
	return nil
}

// Decode a string.
func (o *Buffer) dec_string(p *Properties, base structPointer) error {
	s, err := o.DecodeStringBytes()
	if err != nil {
		return err
	}
	*structPointer_String(base, p.field) = &s
	return nil
}

func (o *Buffer) dec_proto3_string(p *Properties, base structPointer) error {
	s, err := o.DecodeStringBytes()
	if err != nil {
		return err
	}
	*structPointer_StringVal(base, p.field) = s
	return nil
}

// Decode a slice of bytes ([]byte).
func (o *Buffer) dec_slice_byte(p *Properties, base structPointer) error {
	b, err := o.DecodeRawBytes(true)
	if err != nil {
		return err
	}
	*structPointer_Bytes(base, p.field) = b
	return nil
}

// Decode a slice of bools ([]bool).
func (o *Buffer) dec_slice_bool(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	v := structPointer_BoolSlice(base, p.field)
	*v = append(*v, u != 0)
	return nil
}

// Decode a slice of bools ([]bool) in packed format.
func (o *Buffer) dec_slice_packed_bool(p *Properties, base structPointer) error {
	v := structPointer_BoolSlice(base, p.field)

	nn, err := o.DecodeVarint()
	if err != nil {
		return err
	}
	nb := int(nn) // number of bytes of encoded bools
	fin := o.index + nb
	if fin < o.index {
		return errOverflow
	}

	y := *v
	for o.index < fin {
		u, err := p.valDec(o)
		if err != nil {
			return err
		}
		y = append(y, u != 0)
	}

	*v = y
	return nil
}

// Decode a slice of int32s ([]int32).
func (o *Buffer) dec_slice_int32(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	structPointer_Word32Slice(base, p.field).Append(uint32(u))
	return nil
}

// Decode a slice of int32s ([]int32) in packed format.
func (o *Buffer) dec_slice_packed_int32(p *Properties, base structPointer) error {
	v := structPointer_Word32Slice(base, p.field)

	nn, err := o.DecodeVarint()
	if err != nil {
		return err
	}
	nb := int(nn) // number of bytes of encoded int32s

	fin := o.index + nb
	if fin < o.index {
		return errOverflow
	}
	for o.index < fin {
		u, err := p.valDec(o)
		if err != nil {
			return err
		}
		v.Append(uint32(u))
	}
	return nil
}

// Decode a slice of int64s ([]int64).
func (o *Buffer) dec_slice_int64(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}

	structPointer_Word64Slice(base, p.field).Append(u)
	return nil
}

// Decode a slice of int64s ([]int64) in packed format.
func (o *Buffer) dec_slice_packed_int64(p *Properties, base structPointer) error {
	v := structPointer_Word64Slice(base, p.field)

	nn, err := o.DecodeVarint()
	if err != nil {
		return err
	}
	nb := int(nn) // number of bytes of encoded int64s

	fin := o.index + nb
	if fin < o.index {
		return errOverflow
	}
	for o.index < fin {
		u, err := p.valDec(o)
		if err != nil {
			return err
		}
		v.Append(u)
	}
	return nil
}

// Decode a slice of strings ([]string).
func (o *Buffer) dec_slice_string(p *Properties, base structPointer) error {
	s, err := o.DecodeStringBytes()
	if err != nil {
		return err
	}
	v := structPointer_StringSlice(base, p.field)
	*v = append(*v, s)
	return nil
}

// Decode a slice of slice of bytes ([][]byte).
func (o *Buffer) dec_slice_slice_byte(p *Properties, base structPointer) error {
	b, err := o.DecodeRawBytes(true)
	if err != nil {
		return err
	}
	v := structPointer_BytesSlice(base, p.field)
	*v = append(*v, b)
	return nil
}

// Decode a map field.
func (o *Buffer) dec_new_map(p *Properties, base structPointer) error {
	raw, err := o.DecodeRawBytes(false)
	if err != nil {
		return err
	}
	oi := o.index       // index at the end of this map entry
	o.index -= len(raw) // move buffer back to start of map entry

	mptr := structPointer_NewAt(base, p.field, p.mtype) // *map[K]V
	if mptr.Elem().IsNil() {
		mptr.Elem().Set(reflect.MakeMap(mptr.Type().Elem()))
	}
	v := mptr.Elem() // map[K]V

	// Prepare addressable doubly-indirect placeholders for the key and value types.
	// See enc_new_map for why.
	keyptr := reflect.New(reflect.PtrTo(p.mtype.Key())).Elem() // addressable *K
	keybase := toStructPointer(keyptr.Addr())                  // **K

	var valbase structPointer
	var valptr reflect.Value
	switch p.mtype.Elem().Kind() {
	case reflect.Slice:
		// []byte
		var dummy []byte
		valptr = reflect.ValueOf(&dummy)  // *[]byte
		valbase = toStructPointer(valptr) // *[]byte
	case reflect.Ptr:
		// message; valptr is **Msg; need to allocate the intermediate pointer
		valptr = reflect.New(reflect.PtrTo(p.mtype.Elem())).Elem() // addressable *V
		valptr.Set(reflect.New(valptr.Type().Elem()))
		valbase = toStructPointer(valptr)
	default:
		// everything else
		valptr = reflect.New(reflect.PtrTo(p.mtype.Elem())).Elem() // addressable *V
		valbase = toStructPointer(valptr.Addr())                   // **V
	}

	// Decode.
	// This parses a restricted wire format, namely the encoding of a message
	// with two fields. See enc_new_map for the format.
	for o.index < oi {
		// tagcode for key and value properties are always a single byte
		// because they have tags 1 and 2.
		tagcode := o.buf[o.index]
		o.index++
		switch tagcode {
		case p.mkeyprop.tagcode[0]:
			if err := p.mkeyprop.dec(o, p.mkeyprop, keybase); err != nil {
				return err
			}
		case p.mvalprop.tagcode[0]:
			if err := p.mvalprop.dec(o, p.mvalprop, valbase); err != nil {
				return err
			}
		default:
			// TODO: Should we silently skip this instead?
			return fmt.Errorf("proto: bad map data tag %d", raw[0])
		}
	}
	keyelem, valelem := keyptr.Elem(), valptr.Elem()
	if !keyelem.IsValid() {
		keyelem = reflect.Zero(p.mtype.Key())
	}
	if !valelem.IsValid() {
		valelem = reflect.Zero(p.mtype.Elem())
	}

	v.SetMapIndex(keyelem, valelem)
	return nil
}

// Decode a group.
func (o *Buffer) dec_struct_group(p *Properties, base structPointer) error {
	bas := structPointer_GetStructPointer(base, p.field)
	if structPointer_IsNil(bas) {
		// allocate new nested message
		bas = toStructPointer(reflect.New(p.stype))
		structPointer_SetStructPointer(base, p.field, bas)
	}
	return o.unmarshalType(p.stype, p.sprop, true, bas)
}

// Decode an embedded message.
func (o *Buffer) dec_struct_message(p *Properties, base structPointer) (err error) {
	raw, e := o.DecodeRawBytes(false)
	if e != nil {
		return e
	}

	bas := structPointer_GetStructPointer(base, p.field)
	if structPointer_IsNil(bas) {
		// allocate new nested message
		bas = toStructPointer(reflect.New(p.stype))
		structPointer_SetStructPointer(base, p.field, bas)
	}

	// If the object can unmarshal itself, let it.
	if p.isUnmarshaler {
		iv := structPointer_Interface(bas, p.stype)
		return iv.(Unmarshaler).Unmarshal(raw)
	}

	obuf := o.buf
	oi := o.index
	o.buf = raw
	o.index = 0

	err = o.unmarshalType(p.stype, p.sprop, false, bas)
	o.buf = obuf
	o.index = oi

	return err
}

// Decode a slice of embedded messages.
func (o *Buffer) dec_slice_struct_message(p *Properties, base structPointer) error {
	return o.dec_slice_struct(p, false, base)
}

// Decode a slice of embedded groups.
func (o *Buffer) dec_slice_struct_group(p *Properties, base structPointer) error {
	return o.dec_slice_struct(p, true, base)
}

// Decode a slice of structs ([]*struct).
func (o *Buffer) dec_slice_struct(p *Properties, is_group bool, base structPointer) error {
	v := reflect.New(p.stype)
	bas := toStructPointer(v)
	structPointer_StructPointerSlice(base, p.field).Append(bas)

	if is_group {
		err := o.unmarshalType(p.stype, p.sprop, is_group, bas)
		return err
	}

	raw, err := o.DecodeRawBytes(false)
	if err != nil {
		return err
	}

	// If the object can unmarshal itself, let it.
	if p.isUnmarshaler {
		iv := v.Interface()
		return iv.(Unmarshaler).Unmarshal(raw)
	}

	obuf := o.buf
	oi := o.index
	o.buf = raw
	o.index = 0

	err = o.unmarshalType(p.stype, p.sprop, is_group, bas)

	o.buf = obuf
	o.index = oi

	return err
}

// SkipUnrecognized parses and skips any unrecognized tag.
// b is the data stream (after the tag/wiretype has been parsed).
// x is the tag/wiretype decoded from the byte stream.
// u is the place to store the skipped data, or nil if that is not needed.
// Returns the remaining unparsed bytes.
// Note: this is a helper function for the fully custom decoder.
func SkipUnrecognized(b []byte, x uint64, u *[]byte) []byte {
	switch x & 7 {
	case WireVarint:
		_, k := DecodeVarint(b)
		if k == 0 {
			return errorData[:]
		}
		if u != nil {
			*u = encodeVarint(*u, x)
			*u = append(*u, b[:k]...)
		}
		return b[k:]
	case WireFixed32:
		if len(b) < 4 {
			return errorData[:]
		}
		if u != nil {
			*u = encodeVarint(*u, x)
			*u = append(*u, b[:4]...)
		}
		return b[4:]
	case WireFixed64:
		if len(b) < 8 {
			return errorData[:]
		}
		if u != nil {
			*u = encodeVarint(*u, x)
			*u = append(*u, b[:8]...)
		}
		return b[8:]
	case WireBytes:
		m, k := DecodeVarint(b)
		if k == 0 {
			return errorData[:]
		}
		if m+uint64(k) > uint64(len(b)) {
			return errorData[:]
		}
		if u != nil {
			*u = encodeVarint(*u, x)
			*u = append(*u, b[:m+uint64(k)]...)
		}
		return b[m+uint64(k):]
	case WireStartGroup:
		_, j := FindEndGroup(b)
		if j < 0 {
			return errorData[:]
		}
		if u != nil {
			*u = encodeVarint(*u, x)
			*u = append(*u, b[:j]...)
		}
		return b[j:]
	default:
		return errorData[:]
	}
}

// This slice of bytes is an invalid varint.
// Used to indicate an error on return.
var errorData = [...]byte{128}

var errorDataBadWire = [...]byte{129} // bad wire type

// FindEndGroup finds the index of the next EndGroup tag.
// Groups may be nested, so the "next" EndGroup tag is the first
// unpaired EndGroup.
// FindEndGroup returns the indexes of the start and end of the EndGroup tag.
// Returns (-1,-1) if it can't find one.
func FindEndGroup(b []byte) (int, int) {
	depth := 1
	i := 0
	for {
		x, n := DecodeVarint(b[i:])
		if n == 0 {
			return -1, -1
		}
		j := i
		i += n
		switch x & 7 {
		case WireVarint:
			_, k := DecodeVarint(b[i:])
			if k == 0 {
				return -1, -1
			}
			i += k
		case WireFixed32:
			if i+4 > len(b) {
				return -1, -1
			}
			i += 4
		case WireFixed64:
			if i+8 > len(b) {
				return -1, -1
			}
			i += 8
		case WireBytes:
			m, k := DecodeVarint(b[i:])
			if k == 0 {
				return -1, -1
			}
			i += k
			if i+int(m) > len(b) {
				return -1, -1
			}
			i += int(m)
		case WireStartGroup:
			depth++
		case WireEndGroup:
			depth--
			if depth == 0 {
				return j, i
			}
		default:
			return -1, -1
		}
	}
}

type UnmarshalInfo struct {
	typ reflect.Type // type of the struct

	// 0 = only typ field is initialized
	// 1 = completely initialized
	initialized  int32
	lock         sync.Mutex                    // prevents double initialization
	dense        []unmarshalFieldInfo          // fields indexed by tag #
	sparse       map[uint64]unmarshalFieldInfo // fields indexed by tag #
	unrecognized uintptr                       // offset of []byte to put unrecognized data (or 1 if we should throw it away)
	reqMask      uint64                        // mask with a 1 for each required field (up to 64)
}

// An unmarshaler takes a stream of bytes and a pointer to a field of a message.
// It decodes the field and stores it at f and returns the unused bytes.
// w is the wire encoding.
// b is the data after the tag and wire encoding have been read
type unmarshaler func(b []byte, f unsafe.Pointer, w int) ([]byte, error)

type unmarshalFieldInfo struct {
	// offset of the field in the proto message structure.
	offset uintptr

	// function to unmarshal the data for the field.
	unmarshal unmarshaler

	reqMask uint64 // all one bits, execept one zero bit at this field's index in the required field list.
}

func Unmarshal(buf []byte, pb Message) error {
	pb.Reset()
	v := reflect.ValueOf(pb)
	t := v.Type().Elem()
	u := getUnmarshalInfo(t)
	m := unsafe.Pointer(v.Pointer())
	return u.unmarshal(m, buf)
}
func UnmarshalMerge(buf []byte, pb Message) error {
	v := reflect.ValueOf(pb)
	t := v.Type().Elem()
	u := getUnmarshalInfo(t)
	m := unsafe.Pointer(v.Pointer())
	//m := (*[2]unsafe.Pointer)(unsafe.Pointer(&pb))[1]
	return u.unmarshal(m, buf)
}
func (b *Buffer) DecodeMessage(pb Message) error {
	if pb == b.lastMsg {
		return b.lastUnmarshalInfo.unmarshal(b.lastPtr, b.buf)
	}
	v := reflect.ValueOf(pb)
	t := v.Type().Elem()
	u := getUnmarshalInfo(t)
	m := unsafe.Pointer(v.Pointer())
	b.lastMsg = pb
	b.lastPtr = m
	b.lastUnmarshalInfo = u
	return u.unmarshal(m, b.buf)
}
func (b *Buffer) DecodeGroup(pb Message) error {
	panic("DecodeGroup")
}
func (b *Buffer) Unmarshal(pb Message) error {
	if pb == b.lastMsg {
		return b.lastUnmarshalInfo.unmarshal(b.lastPtr, b.buf)
	}
	v := reflect.ValueOf(pb)
	t := v.Type().Elem()
	u := getUnmarshalInfo(t)
	m := unsafe.Pointer(v.Pointer())
	b.lastMsg = pb
	b.lastPtr = m
	b.lastUnmarshalInfo = u
	return u.unmarshal(m, b.buf)
}

// Unmarshal is the entry point from the generated .pb.go files.
// msg contains a pointer to a protocol buffer.
// b is the data to be unmarshaled into the protocol buffer.
// a is a pointer to a place to store cached unmarshal information.
func UnmarshalReflect(msg interface{}, b []byte, a **UnmarshalInfo) error {
	// u := *a, but atomically.
	// We use an atomic here to ensure memory consistency.
	u := (*UnmarshalInfo)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(a))))
	if u == nil {
		// Get unmarshal information from type of message.
		u = getUnmarshalInfo(reflect.ValueOf(msg).Type().Elem())
		// Store it in the cache for later users.
		// *a = u, but atomically.
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(a)), unsafe.Pointer(u))
	}

	// Super-tricky - read pointer out of data word of interface value.
	// Saves ~25ns over the equivalent m := unsafe.Pointer(reflect.ValueOf(msg).Pointer())
	m := (*[2]unsafe.Pointer)(unsafe.Pointer(&msg))[1]

	return u.unmarshal(m, b)
}

// unmarshal does the main work of unmarshaling a message.
// u provides type information used to unmarshal the message.
// m is a pointer to a protocol buffer message.
// b is a byte stream to unmarshal into m.
func (u *UnmarshalInfo) unmarshal(m unsafe.Pointer, b []byte) error {
	if atomic.LoadInt32(&u.initialized) == 0 {
		u.computeUnmarshalInfo()
	}
	reqMask := u.reqMask
	for len(b) > 0 {
		// Read tag and wire type.
		// Special case 1 and 2 byte varints.
		var x uint64
		if b[0] < 128 {
			x = uint64(b[0])
			b = b[1:]
		} else if len(b) >= 2 && b[1] < 128 {
			x = uint64(b[0]&0x7f) + uint64(b[1])<<7
			b = b[2:]
		} else {
			var n int
			x, n = DecodeVarint(b)
			if n == 0 {
				return io.ErrUnexpectedEOF
			}
			b = b[n:]
		}
		tag := x >> 3
		wire := int(x) & 7

		// Dispatch on the tag to one of the unmarshal* functions below.
		var f unmarshalFieldInfo
		if tag < uint64(len(u.dense)) {
			f = u.dense[tag]
		} else {
			f = u.sparse[tag]
		}
		reqMask &= f.reqMask
		if fn := f.unmarshal; fn != nil {
			var err error
			b, err = fn(b, unsafe.Pointer(uintptr(m)+f.offset), wire)
			if err != nil {
				if err == errInternalBadWireType {
					err = fmt.Errorf("bad wiretype for field at offset %d: got wiretype %d", f.offset, wire)
				}
				return err
			}
			continue
		}

		// Unknown tag.
		// TODO: handle extensions here.
		if u.unrecognized == 1 {
			// proto3, don't keep unrecognized data.  Just skip it.
			// Use wire type to skip data.
			switch wire {
			case WireVarint:
				_, k := DecodeVarint(b)
				if len(b) < k {
					return io.ErrUnexpectedEOF
				}
				b = b[k:]
			case WireFixed32:
				if len(b) < 4 {
					return io.ErrUnexpectedEOF
				}
				b = b[4:]
			case WireFixed64:
				if len(b) < 8 {
					return io.ErrUnexpectedEOF
				}
				b = b[8:]
			case WireBytes:
				m, k := DecodeVarint(b)
				if uint64(len(b)) < uint64(k)+m {
					return io.ErrUnexpectedEOF
				}
				b = b[uint64(k)+m:]
			default:
				// WireStartGroup, WireEndGroup not possible for proto3
				return fmt.Errorf("proto: can't skip unknown wire type %d for %s", wire, u.typ)
			}
		} else {
			// proto2, keep unrecognized data around.
			z := (*[]byte)(unsafe.Pointer(uintptr(m) + u.unrecognized))
			*z = encodeVarint(*z, tag<<3|uint64(wire))
			// Use wire type to skip data.
			switch wire {
			case WireVarint:
				_, k := DecodeVarint(b)
				if len(b) < k {
					return io.ErrUnexpectedEOF
				}
				*z = append(*z, b[:k]...)
				b = b[k:]
			case WireFixed32:
				if len(b) < 4 {
					return io.ErrUnexpectedEOF
				}
				*z = append(*z, b[:4]...)
				b = b[4:]
			case WireFixed64:
				if len(b) < 8 {
					return io.ErrUnexpectedEOF
				}
				*z = append(*z, b[:8]...)
				b = b[8:]
			case WireBytes:
				m, k := DecodeVarint(b)
				if uint64(len(b)) < uint64(k)+m {
					return io.ErrUnexpectedEOF
				}
				*z = append(*z, b[:uint64(k)+m]...)
				b = b[uint64(k)+m:]
			case WireStartGroup:
				_, i := FindEndGroup(b)
				if i == -1 {
					return io.ErrUnexpectedEOF
				}
				*z = append(*z, b[:i]...)
				b = b[i:]
			default:
				return fmt.Errorf("proto: can't skip unknown wire type %d for %s", wire, u.typ)
			}
		}
	}
	if reqMask != 0 {
		return &RequiredNotSetError{"{Unknown}"}
	}
	return nil
}

// getUnmarshalInfo returns the data structure which can be
// subsequently used to unmarshal a message of the given type.
// t is the type of the message (note: not pointer to message).
func getUnmarshalInfo(t reflect.Type) *UnmarshalInfo {
	// It would be correct to return a new UnmarshalInfo
	// unconditionally. We would end up allocating one
	// per occurrence of that type as a message or submessage.
	// We use a cache here just to reduce memory usage.
	messageLock.Lock()
	defer messageLock.Unlock()
	u := messageInfo[t]
	if u == nil {
		u = &UnmarshalInfo{typ: t}
		// Note: we just set the type here. The rest of the fields
		// will be initialized on first use.
		messageInfo[t] = u
	}
	return u
}

var messageLock sync.Mutex
var messageInfo = map[reflect.Type]*UnmarshalInfo{}

// computeUnmarshalInfo fills in u with information for use
// in unmarshaling protocol buffers of type u.typ.
func (u *UnmarshalInfo) computeUnmarshalInfo() {
	u.lock.Lock()
	defer u.lock.Unlock()
	if u.initialized != 0 {
		return
	}
	t := u.typ

	// Set up the "not found" value for the unrecognized byte buffer.
	// This is the default for proto3.
	u.unrecognized = 1

	// List of the generated type and offset for each oneof field.
	type oneofField struct {
		ityp   reflect.Type // interface type of oneof field
		offset uintptr      // offset in containing message
	}
	var oneofFields []oneofField

	n := t.NumField()
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.Name == "XXX_unrecognized" {
			// The byte slice used to hold unrecognized input is special.
			u.unrecognized = f.Offset
			continue
		}
		if f.Name == "XXX_InternalExtensions" {
			// TODO: save offset, use for extension processing
			continue
		}
		oneof := f.Tag.Get("protobuf_oneof")
		if oneof != "" {
			oneofFields = append(oneofFields, oneofField{f.Type, f.Offset})
			// The rest of oneof processing happens below.
			continue
		}

		// Extract unmarshaling function from the field (its type and tags).
		unmarshal := fieldUnmarshaler(&f)

		// Required field?
		required := strings.Split(f.Tag.Get("protobuf"), ",")[2] == "req"

		// Store the info in the correct slot in the message.
		tag, err := strconv.Atoi(strings.Split(f.Tag.Get("protobuf"), ",")[1])
		if err != nil {
			panic("protobuf tag field not an integer")
		}
		u.setTag(tag, n, f.Offset, unmarshal, required)
	}

	// Find any types associated with oneof fields.
	// TODO: XXX_OneofFuncs returns more info than we need.  Get rid of some of it?
	fn := reflect.Zero(reflect.PtrTo(t)).MethodByName("XXX_OneofFuncs")
	if fn.IsValid() {
		res := fn.Call(nil)[3] // last return value from XXX_OneofFuncs: []interface{}
		for i := res.Len() - 1; i >= 0; i-- {
			v := res.Index(i)                             // interface{}
			tptr := reflect.ValueOf(v.Interface()).Type() // *Msg_X
			typ := tptr.Elem()                            // Msg_X

			f := typ.Field(0) // oneof implementers have one field
			baseUnmarshal := fieldUnmarshaler(&f)
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
					// when we encounter it during unmarshaling.
					unmarshal := makeUnmarshalOneof(typ, of.ityp, baseUnmarshal)
					u.setTag(tag, n, of.offset, unmarshal, false)
				}
			}
		}
	}

	atomic.StoreInt32(&u.initialized, 1)
}

// setTag stores the unmarshal information for the given tag.
// tag = tag # for field
// n = number of fields
// offset/unmarshal = unmarshal info for that field.
// required = whether the field is required
func (u *UnmarshalInfo) setTag(tag int, n int, offset uintptr, unmarshal unmarshaler, required bool) {
	reqMask := uint64(1<<64 - 1)
	if required {
		// We keep track of the first 64 required fields. We give up after that,
		// so we effectively treat the 65th and beyond required fields as optional.
		reqMask = (1<<64 - 1) - (u.reqMask + 1)
		u.reqMask = u.reqMask*2 + 1
	}
	i := unmarshalFieldInfo{offset: offset, unmarshal: unmarshal, reqMask: reqMask}
	if tag >= 0 && (tag < 16 || tag < 2*n) { // TODO: what are the right numbers here?
		for len(u.dense) <= tag {
			u.dense = append(u.dense, unmarshalFieldInfo{})
		}
		u.dense[tag] = i
	} else {
		if u.sparse == nil {
			u.sparse = map[uint64]unmarshalFieldInfo{}
		}
		u.sparse[uint64(tag)] = i
	}

}

// fieldUnmarshaler returns an unmarshaler for the given field.
func fieldUnmarshaler(f *reflect.StructField) unmarshaler {
	if f.Type.Kind() == reflect.Map {
		return makeUnmarshalMap(f)
	}
	return typeUnmarshaler(f.Type, f.Tag.Get("protobuf"))
}

// typeUnmarshaler returns an unmarshaler for the given field type / field tag pair.
func typeUnmarshaler(t reflect.Type, tags string) unmarshaler {
	tagArray := strings.Split(tags, ",")
	encoding := tagArray[0]
	var name = "unknown"
	if len(tagArray) >= 4 && strings.HasPrefix(tagArray[3], "name=") {
		name = tagArray[3][5:]
	}

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

	switch t.Kind() {
	case reflect.Bool:
		if pointer {
			return unmarshalBoolPtr
		}
		if slice {
			return unmarshalBoolSlice
		}
		return unmarshalBoolValue
	case reflect.Int32:
		switch encoding {
		case "varint":
			// this could be int32 or enum
			if pointer {
				return unmarshalInt32Ptr
			}
			if slice {
				return unmarshalInt32Slice
			}
			return unmarshalInt32Value
		case "zigzag32":
			if pointer {
				return unmarshalSint32Ptr
			}
			if slice {
				return unmarshalSint32Slice
			}
			return unmarshalSint32Value
		}
	case reflect.Int64:
		switch encoding {
		case "varint":
			if pointer {
				return unmarshalInt64Ptr
			}
			if slice {
				return unmarshalInt64Slice
			}
			return unmarshalInt64Value
		case "zigzag64":
			if pointer {
				return unmarshalSint64Ptr
			}
			if slice {
				return unmarshalSint64Slice
			}
			return unmarshalSint64Value
		}
	case reflect.Uint32:
		switch encoding {
		case "fixed32":
			if pointer {
				return unmarshalFixed32Ptr
			}
			if slice {
				return unmarshalFixed32Slice
			}
			return unmarshalFixed32Value
		case "varint":
			if pointer {
				return unmarshalInt32Ptr
			}
			if slice {
				return unmarshalInt32Slice
			}
			return unmarshalInt32Value
		}
	case reflect.Uint64:
		switch encoding {
		case "fixed64":
			if pointer {
				return unmarshalFixed64Ptr
			}
			if slice {
				return unmarshalFixed64Slice
			}
			return unmarshalFixed64Value
		case "varint":
			if pointer {
				return unmarshalInt64Ptr
			}
			if slice {
				return unmarshalInt64Slice
			}
			return unmarshalInt64Value
		}
	case reflect.Float32:
		if pointer {
			return unmarshalFloat32Ptr
		}
		if slice {
			return unmarshalFloat32Slice
		}
		return unmarshalFloat32Value
	case reflect.Float64:
		if pointer {
			return unmarshalFloat64Ptr
		}
		if slice {
			return unmarshalFloat64Slice
		}
		return unmarshalFloat64Value
	case reflect.Map:
		panic("map type in typeUnmarshaler")
	case reflect.Slice:
		if pointer {
			panic("bad pointer in slice case")
		}
		if slice {
			return unmarshalBytesSlice
		}
		return unmarshalBytesValue
	case reflect.String:
		if pointer {
			return unmarshalStringPtr
		}
		if slice {
			return unmarshalStringSlice
		}
		return unmarshalStringValue
	case reflect.Struct:
		// message or group field
		if !pointer {
			panic(fmt.Sprintf("message/group field %s:%s without pointer", t, encoding))
		}
		switch encoding {
		case "bytes":
			if slice {
				return makeUnmarshalMessageSlicePtr(getUnmarshalInfo(t), name)
			}
			return makeUnmarshalMessagePtr(getUnmarshalInfo(t), name)
		case "group":
			if slice {
				return makeUnmarshalGroupSlicePtr(getUnmarshalInfo(t), name)
			}
			return makeUnmarshalGroupPtr(getUnmarshalInfo(t), name)
		}
	}
	panic(fmt.Sprintf("unmarshaler not found type:%s encoding:%s", t, encoding))
}

// Below are all the unmarshalers for individual fields of various types.

func unmarshalFloat64Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float64frombits(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)
	g := (*float64)(f)
	*g = v
	return b[8:], nil
}

func unmarshalFloat64Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float64frombits(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)
	g := (**float64)(f)
	*g = &v
	return b[8:], nil
}

func unmarshalFloat64Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]float64)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			if len(b) < 8 {
				return nil, io.ErrUnexpectedEOF
			}
			v := math.Float64frombits(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)
			*g = append(*g, v)
			b = b[8:]
		}
		return res, nil
	}
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float64frombits(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)
	*g = append(*g, v)
	return b[8:], nil
}

func unmarshalFloat32Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float32frombits(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
	g := (*float32)(f)
	*g = v
	return b[4:], nil
}

func unmarshalFloat32Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float32frombits(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
	g := (**float32)(f)
	*g = &v
	return b[4:], nil
}

func unmarshalFloat32Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]float32)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			if len(b) < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			v := math.Float32frombits(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
			*g = append(*g, v)
			b = b[4:]
		}
		return res, nil
	}
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := math.Float32frombits(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
	*g = append(*g, v)
	return b[4:], nil
}

func unmarshalInt64Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x)
	g := (*int64)(f)
	*g = v
	return b, nil
}

func unmarshalInt64Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x)
	g := (**int64)(f)
	*g = &v
	return b, nil
}

func unmarshalInt64Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]int64)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			x, n = DecodeVarint(b)
			if n == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			b = b[n:]
			v := int64(x)
			*g = append(*g, v)
		}
		return res, nil
	}
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x)
	*g = append(*g, v)
	return b, nil
}

func unmarshalSint64Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x>>1) ^ int64(x)<<63>>63
	g := (*int64)(f)
	*g = v
	return b, nil
}

func unmarshalSint64Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x>>1) ^ int64(x)<<63>>63
	g := (**int64)(f)
	*g = &v
	return b, nil
}

func unmarshalSint64Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]int64)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			x, n = DecodeVarint(b)
			if n == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			b = b[n:]
			v := int64(x>>1) ^ int64(x)<<63>>63
			*g = append(*g, v)
		}
		return res, nil
	}
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int64(x>>1) ^ int64(x)<<63>>63
	*g = append(*g, v)
	return b, nil
}

func unmarshalInt32Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x)
	g := (*int32)(f)
	*g = v
	return b, nil
}

func unmarshalInt32Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x)
	g := (**int32)(f)
	*g = &v
	return b, nil
}

func unmarshalInt32Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]int32)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			x, n = DecodeVarint(b)
			if n == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			b = b[n:]
			v := int32(x)
			*g = append(*g, v)
		}
		return res, nil
	}
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x)
	*g = append(*g, v)
	return b, nil
}

func unmarshalSint32Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x>>1) ^ int32(x)<<31>>31
	g := (*int32)(f)
	*g = v
	return b, nil
}

func unmarshalSint32Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x>>1) ^ int32(x)<<31>>31
	g := (**int32)(f)
	*g = &v
	return b, nil
}

func unmarshalSint32Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]int32)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			x, n = DecodeVarint(b)
			if n == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			b = b[n:]
			v := int32(x>>1) ^ int32(x)<<31>>31
			*g = append(*g, v)
		}
		return res, nil
	}
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	v := int32(x>>1) ^ int32(x)<<31>>31
	*g = append(*g, v)
	return b, nil
}

func unmarshalBoolValue(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	if len(b) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	v := false
	if b[0] != 0 {
		v = true
	}
	g := (*bool)(f)
	*g = v
	return b[1:], nil
}

func unmarshalBoolPtr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	if len(b) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	v := false
	if b[0] != 0 {
		v = true
	}
	g := (**bool)(f)
	*g = &v
	return b[1:], nil
}

func unmarshalBoolSlice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]bool)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			v := false
			if b[0] != 0 {
				v = true
			}
			*g = append(*g, v)
			b = b[1:]
		}
		return res, nil
	}
	if w != WireVarint {
		return nil, errInternalBadWireType
	}
	if len(b) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	v := false
	if b[0] != 0 {
		v = true
	}
	*g = append(*g, v)
	return b[1:], nil
}

func unmarshalFixed64Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	g := (*uint64)(f)
	*g = v
	return b[8:], nil
}

func unmarshalFixed64Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	g := (**uint64)(f)
	*g = &v
	return b[8:], nil
}

func unmarshalFixed64Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]uint64)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			if len(b) < 8 {
				return nil, io.ErrUnexpectedEOF
			}
			v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
			*g = append(*g, v)
			b = b[8:]
		}
		return res, nil
	}
	if w != WireFixed64 {
		return nil, errInternalBadWireType
	}
	if len(b) < 8 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	*g = append(*g, v)
	return b[8:], nil
}

func unmarshalFixed32Value(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	g := (*uint32)(f)
	*g = v
	return b[4:], nil
}

func unmarshalFixed32Ptr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	g := (**uint32)(f)
	*g = &v
	return b[4:], nil
}

func unmarshalFixed32Slice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	g := (*[]uint32)(f)
	if w == WireBytes { // packed
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		res := b[x:]
		b = b[:x]
		for len(b) > 0 {
			if len(b) < 4 {
				return nil, io.ErrUnexpectedEOF
			}
			v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
			*g = append(*g, v)
			b = b[4:]
		}
		return res, nil
	}
	if w != WireFixed32 {
		return nil, errInternalBadWireType
	}
	if len(b) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	*g = append(*g, v)
	return b[4:], nil
}

func unmarshalStringValue(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireBytes {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	if x > uint64(len(b)) {
		return nil, io.ErrUnexpectedEOF
	}
	v := string(b[:x])
	g := (*string)(f)
	*g = v
	return b[x:], nil
}

func unmarshalStringPtr(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireBytes {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	if x > uint64(len(b)) {
		return nil, io.ErrUnexpectedEOF
	}
	v := string(b[:x])
	g := (**string)(f)
	*g = &v
	return b[x:], nil
}

func unmarshalStringSlice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireBytes {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	if x > uint64(len(b)) {
		return nil, io.ErrUnexpectedEOF
	}
	v := string(b[:x])
	g := (*[]string)(f)
	*g = append(*g, v)
	return b[x:], nil
}

func unmarshalBytesValue(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireBytes {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	if x > uint64(len(b)) {
		return nil, io.ErrUnexpectedEOF
	}
	v := make([]byte, x)
	copy(v, b)
	g := (*[]byte)(f)
	*g = v
	return b[x:], nil
}

func unmarshalBytesSlice(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
	if w != WireBytes {
		return nil, errInternalBadWireType
	}
	x, n := DecodeVarint(b)
	if n == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	b = b[n:]
	if x > uint64(len(b)) {
		return nil, io.ErrUnexpectedEOF
	}
	v := make([]byte, x)
	copy(v, b)
	g := (*[][]byte)(f)
	*g = append(*g, v)
	return b[x:], nil
}

func makeUnmarshalMessagePtr(sub *UnmarshalInfo, name string) unmarshaler {
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		if w != WireBytes {
			return nil, errInternalBadWireType
		}
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		g := (*unsafe.Pointer)(f)
		v := *g
		if v == nil {
			v = unsafe.Pointer(reflect.New(sub.typ).Pointer())
			*g = v
		}
		err := sub.unmarshal(v, b[:x])
		if err != nil {
			if rnse, ok := err.(*RequiredNotSetError); ok {
				rnse.field = name + "." + rnse.field
			}
			return nil, err
		}
		return b[x:], nil
	}
}

func makeUnmarshalMessageSlicePtr(sub *UnmarshalInfo, name string) unmarshaler {
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		if w != WireBytes {
			return nil, errInternalBadWireType
		}
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		v := unsafe.Pointer(reflect.New(sub.typ).Pointer())
		err := sub.unmarshal(v, b[:x])
		if err != nil {
			if rnse, ok := err.(*RequiredNotSetError); ok {
				rnse.field = name + "." + rnse.field
			}
			return nil, err
		}
		g := (*[]unsafe.Pointer)(f)
		*g = append(*g, v)
		return b[x:], nil
	}
}

func makeUnmarshalGroupPtr(sub *UnmarshalInfo, name string) unmarshaler {
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		if w != WireStartGroup {
			return nil, errInternalBadWireType
		}
		x, y := FindEndGroup(b)
		if x < 0 {
			return nil, io.ErrUnexpectedEOF
		}
		v := unsafe.Pointer(reflect.New(sub.typ).Pointer())
		err := sub.unmarshal(v, b[:x])
		if err != nil {
			if rnse, ok := err.(*RequiredNotSetError); ok {
				rnse.field = name + "." + rnse.field
			}
			return nil, err
		}
		g := (*unsafe.Pointer)(f)
		*g = v
		return b[y:], nil
	}
}

func makeUnmarshalGroupSlicePtr(sub *UnmarshalInfo, name string) unmarshaler {
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		if w != WireStartGroup {
			return nil, errInternalBadWireType
		}
		x, y := FindEndGroup(b)
		if x < 0 {
			return nil, io.ErrUnexpectedEOF
		}
		v := unsafe.Pointer(reflect.New(sub.typ).Pointer())
		err := sub.unmarshal(v, b[:x])
		if err != nil {
			if rnse, ok := err.(*RequiredNotSetError); ok {
				rnse.field = name + "." + rnse.field
			}
			return nil, err
		}
		g := (*[]unsafe.Pointer)(f)
		*g = append(*g, v)
		return b[y:], nil
	}
}

func makeUnmarshalMap(f *reflect.StructField) unmarshaler {
	t := f.Type
	unmarshalKey := typeUnmarshaler(f.Type.Key(), f.Tag.Get("protobuf_key"))
	unmarshalVal := typeUnmarshaler(f.Type.Elem(), f.Tag.Get("protobuf_val"))
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		// The map entry is a submessage. Figure out how big it is.
		if w != WireBytes {
			return nil, fmt.Errorf("bad wiretype for map field: got %d want %d", w, WireBytes)
		}
		x, n := DecodeVarint(b)
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		b = b[n:]
		if x > uint64(len(b)) {
			return nil, io.ErrUnexpectedEOF
		}
		r := b[x:] // unused data to return
		b = b[:x]  // data for map entry

		// Note: we could use #keys * #values ~= 200 functions
		// to do map decoding without reflection. Probably not worth it.
		// Maps will be somewhat slow. Oh well.

		// Read key and value from data.
		k := reflect.New(t.Key())
		v := reflect.New(t.Elem())
		for len(b) > 0 {
			x, n := DecodeVarint(b)
			if n == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			b = b[n:]
			switch x >> 3 {
			case 1:
				var err error
				b, err = unmarshalKey(b, unsafe.Pointer(k.Pointer()), int(x)&7)
				if err != nil {
					return nil, err
				}
			case 2:
				var err error
				b, err = unmarshalVal(b, unsafe.Pointer(v.Pointer()), int(x)&7)
				if err != nil {
					return nil, err
				}
			default:
				// TODO: skip unknown tags
			}
		}

		// Get map, allocate if needed.
		m := reflect.NewAt(t, f).Elem() // an addressable map[K]T
		if m.IsNil() {
			m.Set(reflect.MakeMap(t))
		}

		// Insert into map.
		m.SetMapIndex(k.Elem(), v.Elem())

		return r, nil
	}
}

// makeUnmarshalOneof makes an unmarshaler for oneof fields.
// for:
// message Msg {
//   oneof F {
//     int64 X = 1;
//     float64 Y = 2;
//   }
// }
// typ is the type of the concrete entry for a oneof case (e.g. Msg_X).
// ityp is the interface type of the oneof field (e.g. isMsg_F).
// unmarshal is the unmarshaler for the base type of the oneof case (e.g. int64).
// Note that this function will be called once for each case in the oneof.
func makeUnmarshalOneof(typ, ityp reflect.Type, unmarshal unmarshaler) unmarshaler {
	return func(b []byte, f unsafe.Pointer, w int) ([]byte, error) {
		// Allocate holder for value.
		v := reflect.New(typ)

		// Unmarshal data into holder.
		// The holder only has one field, so the field location is
		// the same as the holder itself.
		var err error
		b, err = unmarshal(b, unsafe.Pointer(v.Pointer()), w)
		if err != nil {
			return nil, err
		}

		// Write pointer to holder into target field.
		reflect.NewAt(ityp, f).Elem().Set(v)

		return b, nil
	}
}
