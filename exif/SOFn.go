package ImgMeta

import (
	"encoding/binary"
	"fmt"
)

type tSOFnAPP struct {
	marker uint16
	endian binary.ByteOrder
	block  []byte
}

func (t tSOFnAPP) Name() string {
	return fmt.Sprintf("SOF%01X", t.Marker()&0x0F)
}
func (t tSOFnAPP) Marker() uint16 {
	return t.marker
}
func (t tSOFnAPP) Length() uint16 {
	return t.endian.Uint16(t.block[2:])
}
func (t tSOFnAPP) ID(cid []byte) []byte {
	return []byte{}
}
func (t tSOFnAPP) HasID(cid []byte) bool {
	return true
}

func (t tSOFnAPP) ReadValue(tagID2Find uint16) (interface{}, error) {
	if t.Marker()&0x0F == 0 {
		if tagID2Find == SOF0ImageBPP {
			return uint32(t.block[SOF0ImageBPP]), nil
		} else if tagID2Find == SOF0ImageHeight {
			return uint32(t.endian.Uint16(t.block[SOF0ImageHeight : SOF0ImageHeight+2])), nil
		} else if tagID2Find == SOF0ImageWidth {
			return uint32(t.endian.Uint16(t.block[SOF0ImageWidth : SOF0ImageWidth+2])), nil
		}
	}
	return int(0), nil
}

const (
	SOF0ImageBPP    = 0x0004
	SOF0ImageHeight = 0x0005
	SOF0ImageWidth  = 0x0007
)
