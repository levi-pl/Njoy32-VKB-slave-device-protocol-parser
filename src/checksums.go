package main

import (
	"encoding/binary"
	"fmt"

	"github.com/snksoft/crc"
)

// XOR Checksum (LRC) - Simplest, used in NMEA
func XORChecksum(data []byte) byte {
	var checksum byte
	for _, b := range data {
		checksum ^= b
	}
	return checksum
}

// Additive 8-bit Checksum - Used in XBee and simple UART
func Additive8(data []byte) byte {
	var sum uint8
	for _, b := range data {
		sum += b
	}
	return sum
}

// Fletcher-16 - High error detection, low CPU cost
func Fletcher16(data []byte) uint16 {
	var sum1, sum2 uint16
	for _, b := range data {
		sum1 = (sum1 + uint16(b)) % 255
		sum2 = (sum2 + sum1) % 255
	}
	return (sum2 << 8) | sum1
}

// Internet Checksum (RFC 1071) - Used in TCP/IP
func InternetChecksum(data []byte) uint16 {
	var sum uint32

	// Process as 16-bit words
	for i := 0; i < len(data)-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}

	// Handle odd byte
	if len(data)%2 != 0 {
		sum += uint32(data[len(data)-1]) << 8
	}

	// Wrap carries
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return ^uint16(sum)
}

func calculateVariousChecksums(data []byte) {
	fmt.Print("crc8: ")
	for _, code := range *crc8 {
		fmt.Printf("[%02X]", crc.CalculateCRC(&code, data[:len(data)-1]))
		fmt.Printf("[%02X]", crc.CalculateCRC(&code, data[1:len(data)-1]))
		fmt.Printf("[%02X]", crc.CalculateCRC(&code, data[4:len(data)-1]))
	}
	fmt.Println()
	fmt.Print("crc16: ")
	for _, code := range crc16 {
		fmt.Printf("[%04X]", crc.CalculateCRC(code, data[:len(data)-2]))
		fmt.Printf("[%04X]", crc.CalculateCRC(code, data[1:len(data)-2]))
		fmt.Printf("[%04X]", crc.CalculateCRC(code, data[4:len(data)-2]))
	}
	fmt.Println()
	fmt.Print("XOR: ")
	fmt.Printf("[%02X]", XORChecksum(data[:len(data)-1]))
	fmt.Printf("[%02X]", XORChecksum(data[1:len(data)-1]))
	fmt.Printf("[%02X]", XORChecksum(data[4:len(data)-1]))
	fmt.Print(" Additive8: ")
	fmt.Printf("[%02X]", Additive8(data[:len(data)-1]))
	fmt.Printf("[%02X]", Additive8(data[1:len(data)-1]))
	fmt.Printf("[%02X]", Additive8(data[4:len(data)-1]))
	fmt.Print(" Fletcher16: ")
	fmt.Printf("[%04X]", Fletcher16(data[:len(data)-2]))
	fmt.Printf("[%04X]", Fletcher16(data[1:len(data)-2]))
	fmt.Printf("[%04X]", Fletcher16(data[4:len(data)-2]))
	fmt.Println()
}
