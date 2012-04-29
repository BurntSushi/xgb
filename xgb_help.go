package xgb

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

func (c *Conn) bytesStrList(list []Str, length int) []byte {
	buf := make([]byte, 0)
	for _, str := range list {
		buf = append(buf, []byte(str.Name)...)
	}
	return c.bytesPadding(buf)
}

func (c *Conn) bytesUInt32List(list []uint32) []byte {
	buf := make([]byte, len(list)*4)
	for i, item := range list {
		put32(buf[i*4:], item)
	}
	return c.bytesPadding(buf)
}

func (c *Conn) bytesIdList(list []Id, length int) []byte {
	buf := make([]byte, length*4)
	for i, item := range list {
		put32(buf[i*4:], uint32(item))
	}
	return c.bytesPadding(buf)
}

// Pad a length to align on 4 bytes.
func pad(n int) int { return (n + 3) & ^3 }

func put16(buf []byte, v uint16) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
}

func put32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

func get16(buf []byte) uint16 {
	v := uint16(buf[0])
	v |= uint16(buf[1]) << 8
	return v
}

func get32(buf []byte) uint32 {
	v := uint32(buf[0])
	v |= uint32(buf[1]) << 8
	v |= uint32(buf[2]) << 16
	v |= uint32(buf[3]) << 24
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

// ClientMessageData holds the data from a client message,
// duplicated in three forms because Go doesn't have unions.
type ClientMessageData struct {
	Data8  [20]byte
	Data16 [10]uint16
	Data32 [5]uint32
}

func getClientMessageData(b []byte, v *ClientMessageData) int {
	copy(v.Data8[:], b)
	for i := 0; i < 10; i++ {
		v.Data16[i] = get16(b[i*2:])
	}
	for i := 0; i < 5; i++ {
		v.Data32[i] = get32(b[i*4:])
	}
	return 20
}
