package client

import (
	"encoding/binary"
	"net"
	"fmt"
)

var crc32Table []uint32

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

type packet struct {
	packet_version  uint16
	packet_type     uint16
	crc32_value     uint32
	result_code     uint16
	buffer        []byte
}

func init() {

	var crc, poly uint32
	var i, j uint32

	crc32Table = make([]uint32, 256)

	poly = uint32(0xEDB88320)

    for i=0; i<256; i++ {
        crc = i

        for j=8; j>0; j-- {
            if (crc & 1) != 0 {
                crc = (crc>>1) ^ poly
			} else {
                crc>>=1
			}
		}

        crc32Table[i] = crc
	}

}

func crc32(in []byte) uint32 {
	var crc uint32

	crc = uint32(0xFFFFFFFF)

	for _, c := range in {
		crc = ((crc>>8) & uint32(0x00FFFFFF)) ^ crc32Table[(crc ^ uint32(c)) & 0xFF]
	}

	return (crc ^ uint32(0xFFFFFFFF));
}

func CreatePacket(command string) (*packet) {
	var p packet

	p.buffer = make([]byte, MAX_PACKETBUFFER_LENGTH)

	p.packet_version = NRPE_PACKET_VERSION_2
	p.packet_type    = QUERY_PACKET
	copy(p.buffer, []byte(command))

	return &p
}

func (p *packet) Len() int {
	return MAX_PACKETBUFFER_LENGTH + 12
}

func SendPacket(conn net.Conn, pkt *packet) error {

	be := binary.BigEndian

	buffer := make([]byte, pkt.Len())

	p := buffer

	/*
	buffer structure
	| 16 bit version | 16 bit type | 32 bit crc | 1024 byte data |
	|       |        |     |       |   |   |  | | | | | | | ...  |
	*/

	be.PutUint16(p, pkt.packet_version)
	p = p[2:]

	be.PutUint16(p, pkt.packet_type)
	p = p[2:]

	be.PutUint32(p, 0)
	p = p[4:]

	be.PutUint16(p, 3)
	p = p[2:]

	copy(p, pkt.buffer)

	crc := crc32(buffer)

	be.PutUint32(buffer[4:], crc)

	l, err := conn.Write(buffer)

	fmt.Printf("%+v\n", buffer)

	if err != nil || l != len(buffer) {
		return err
	}

	respBuffer := make([]byte, pkt.Len())

	l, err = conn.Read(respBuffer)

	if err != nil || l != len(respBuffer) {
		return err
	}

	var resp packet

	resp.packet_version = be.Uint16(respBuffer)
	resp.packet_type    = be.Uint16(respBuffer[2:])
	resp.crc32_value    = be.Uint32(respBuffer[4:])
	resp.result_code    = be.Uint16(respBuffer[8:])

	resp.buffer = respBuffer[10:]

	fmt.Printf("%s\n", string(resp.buffer))

	return nil
}
