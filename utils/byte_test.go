package utils

import (
	"bytes"
	"testing"
)

func TestTLBytes(t *testing.T) {
	buf := []byte{0xFF, 0xAA, 0xCC}
	if !bytes.Equal(append([]byte{3}, buf...), TLBytes(buf)) {
		t.Fatal("not equal small")
		return
	}

	buf = buf[:0]
	for i := 0; i < 254; i++ {
		buf = append(buf, 0xFF)
	}

	// corner case + round to 4
	if !bytes.Equal(append([]byte{0xFE, 0xFE, 0x00, 0x00}, append(buf, 0x00, 0x00)...), TLBytes(buf)) {
		t.Fatal("not equal middle")
		return
	}

	buf = buf[:0]
	for i := 0; i < 1217; i++ {
		buf = append(buf, byte(i%256))
	}

	if !bytes.Equal(append([]byte{0xFE, 0xC1, 0x04, 0x00}, append(buf, 0x00, 0x00, 0x00)...), TLBytes(buf)) {
		t.Fatal("not equal big")
		return
	}
}
