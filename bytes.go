package sessionid

import (
	"reflect"
	"unsafe"
)

func ToBytes(str string) []byte {
	strHeader := (*reflect.StringHeader)(unsafe.Pointer(&str))
	var b []byte
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sliceHeader.Data = strHeader.Data
	sliceHeader.Len = strHeader.Len
	sliceHeader.Cap = strHeader.Len
	return b
}

func ToString(b []byte) string {
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	var str string
	strHeader := (*reflect.StringHeader)(unsafe.Pointer(&str))
	strHeader.Data = sliceHeader.Data
	strHeader.Len = sliceHeader.Len
	return str
}
