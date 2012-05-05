package xgb

import (
	"fmt"
	"strings"
)

// stringsJoin is an alias to strings.Join. It allows us to avoid having to
// import 'strings' in each of the generated Go files.
func stringsJoin(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// sprintf is so we don't need to import 'fmt' in the generated Go files.
func sprintf(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}

// Pad a length to align on 4 bytes.
func pad(n int) int {
	return (n + 3) & ^3
}

// popCount counts the number of bits set in a value list mask.
func popCount(mask0 int) int {
	mask := uint32(mask0)
	n := 0
	for i := uint32(0); i < 32; i++ {
		if mask&(1<<i) != 0 {
			n++
		}
	}
	return n
}

// Put16 takes a 16 bit integer and copies it into a byte slice.
func Put16(buf []byte, v uint16) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
}

// Put32 takes a 32 bit integer and copies it into a byte slice.
func Put32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

// Put64 takes a 64 bit integer and copies it into a byte slice.
func Put64(buf []byte, v uint64) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
	buf[4] = byte(v >> 32)
	buf[5] = byte(v >> 40)
	buf[6] = byte(v >> 48)
	buf[7] = byte(v >> 56)
}

// Get16 constructs a 16 bit integer from the beginning of a byte slice.
func Get16(buf []byte) uint16 {
	v := uint16(buf[0])
	v |= uint16(buf[1]) << 8
	return v
}

// Get32 constructs a 32 bit integer from the beginning of a byte slice.
func Get32(buf []byte) uint32 {
	v := uint32(buf[0])
	v |= uint32(buf[1]) << 8
	v |= uint32(buf[2]) << 16
	v |= uint32(buf[3]) << 24
	return v
}

// Get64 constructs a 64 bit integer from the beginning of a byte slice.
func Get64(buf []byte) uint64 {
	v := uint64(buf[0])
	v |= uint64(buf[1]) << 8
	v |= uint64(buf[2]) << 16
	v |= uint64(buf[3]) << 24
	v |= uint64(buf[4]) << 32
	v |= uint64(buf[5]) << 40
	v |= uint64(buf[6]) << 48
	v |= uint64(buf[7]) << 56
	return v
}
