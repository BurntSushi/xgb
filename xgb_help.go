package xgb

import (
	"fmt"
	"strings"
)

// getExtensionOpcode retrieves the extension opcode from the extensions map.
// If one doesn't exist, just return 0. An X error will likely result.
func (c *Conn) getExtensionOpcode(name string) byte {
	return c.extensions[name]
}

func (c *Conn) bytesPadding(buf []byte) []byte {
	return append(buf, make([]byte, pad(len(buf))-len(buf))...)
}

func (c *Conn) bytesString(str string) []byte {
	return c.bytesPadding([]byte(str))
}

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
func pad(n int) int { return (n + 3) & ^3 }

func Put16(buf []byte, v uint16) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
}

func Put32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

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

func Get16(buf []byte) uint16 {
	v := uint16(buf[0])
	v |= uint16(buf[1]) << 8
	return v
}

func Get32(buf []byte) uint32 {
	v := uint32(buf[0])
	v |= uint32(buf[1]) << 8
	v |= uint32(buf[2]) << 16
	v |= uint32(buf[3]) << 24
	return v
}

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

// Voodoo to count the number of bits set in a value list mask.
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

// DefaultScreen returns the Screen info for the default screen, which is
// 0 or the one given in the display argument to Dial.
func (c *Conn) DefaultScreen() *ScreenInfo { return &c.Setup.Roots[c.defaultScreen] }
