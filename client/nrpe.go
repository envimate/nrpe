package client

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
	"unsafe"
)

/*
#include <stdint.h>

#define MAX_PACKETBUFFER_LENGTH 1024

struct nrpe_packet {
	int16_t  packet_version;
	int16_t  packet_type;
	uint32_t crc32_value;
	int16_t  result_code;
	char     buffer[MAX_PACKETBUFFER_LENGTH];
};
*/
import "C"

var crc32Table []uint32

var randSource *rand.Rand

const (
	MAX_PACKETBUFFER_LENGTH = 1024
)

const (
	QUERY_PACKET    = 1
	RESPONSE_PACKET = 2
)

const (
	NRPE_PACKET_VERSION_3 = 3
	NRPE_PACKET_VERSION_2 = 2
	NRPE_PACKET_VERSION_1 = 1
)

func init() {
	var crc, poly, i, j uint32

	crc32Table = make([]uint32, 256)

	poly = uint32(0xEDB88320)

	for i = 0; i < 256; i++ {
		crc = i

		for j = 8; j > 0; j-- {
			if (crc & 1) != 0 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}

		crc32Table[i] = crc
	}

	randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func crc32(in []byte) uint32 {
	var crc uint32

	crc = uint32(0xFFFFFFFF)

	for _, c := range in {
		crc = ((crc >> 8) & uint32(0x00FFFFFF)) ^ crc32Table[(crc^uint32(c))&0xFF]
	}

	return (crc ^ uint32(0xFFFFFFFF))
}

func randomizeBuffer(in []byte) {
	n := len(in) >> 2

	for i := 0; i < n; i++ {
		r := randSource.Uint32()

		copy(in[i<<2:(i+1)<<2], (*[4]byte)(unsafe.Pointer(&r))[:])
	}

	if len(in)%4 != 0 {
		r := randSource.Uint32()

		copy(in[n<<2:len(in)], (*[4]byte)(unsafe.Pointer(&r))[:len(in)-(n<<2)])
	}
}

func SendRequest(conn net.Conn, command string) (*string, error) {
	return SendRequestWithArgs(conn, command, nil)
}

func SendRequestWithArgs(conn net.Conn, command string, args []string) (*string, error) {
	var pkt C.struct_nrpe_packet

	b := (*(*[1<<31 - 1]byte)(unsafe.Pointer(&pkt)))[:unsafe.Sizeof(pkt)]

	be := binary.BigEndian

	randomizeBuffer(b)

	be.PutUint16(b[unsafe.Offsetof(pkt.packet_version):], NRPE_PACKET_VERSION_2)
	be.PutUint16(b[unsafe.Offsetof(pkt.packet_type):], QUERY_PACKET)
	be.PutUint32(b[unsafe.Offsetof(pkt.crc32_value):], 0)
	be.PutUint16(b[unsafe.Offsetof(pkt.result_code):], 3)

	copy(b[unsafe.Offsetof(pkt.buffer):], []byte(command))
	b[int(unsafe.Offsetof(pkt.buffer))+len(command)] = 0

	be.PutUint32(b[unsafe.Offsetof(pkt.crc32_value):], crc32(b))

	bb := make([]byte, len(b))
	copy(bb, b)

	respBuffer := make([]byte, len(b))

	if true {
		err := sendSSL(conn, bb, respBuffer)

		if err != nil {
			return nil, err
		}

	} else {
		l, err := conn.Write(bb)

		if err != nil || l != len(bb) {
			return nil, err
		}

		l, err = conn.Read(respBuffer)

		if err != nil || l != len(respBuffer) {
			return nil, err
		}
	}

	var resp C.struct_nrpe_packet

	r := (*(*[1<<31 - 1]byte)(unsafe.Pointer(&resp)))[:unsafe.Sizeof(resp)]

	copy(r, respBuffer)

	crc := be.Uint32(r[unsafe.Offsetof(resp.crc32_value):])

	be.PutUint32(r[unsafe.Offsetof(resp.crc32_value):], 0)

	if crc32(r) != crc {
		return nil, fmt.Errorf("crc didnt match")
	}

	res := C.GoString((*C.char)(unsafe.Pointer(&resp.buffer[0])))

	return &res, nil
}
