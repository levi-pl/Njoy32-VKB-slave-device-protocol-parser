package main

import (
	"github.com/snksoft/crc"
	"go.bug.st/serial"
)

type (
	Njoy32MessageDescriptor struct {
		Size int
		Name string
	}

	Njoy32DeviceDescriptor struct {
		BaseAddress uint8
		MaxCount    uint8
		Model       string
		Description string
	}

	Njoy32Device struct {
		Model string
		Index uint8
	}

	Njoy32Size struct {
		Header  int
		Payload int
	}
	Njoy32MessageType struct {
		Command    int
		SubCommand int
	}

	Njoy32Message struct {
		Response bool
		Known    bool
		Size     Njoy32Size
		Type     Njoy32MessageType
		Device   Njoy32Device
		RawData  []byte
	}

	PacketState int
)

const (
	maxPacketLength = 64
	startOfPacket   = 0x5A
	syncByte        = 0xAA
	endPreamble     = 0xA5

	StateUndefined PacketState = iota
	StateSOP
	StateFirstSync
	StateSecondSync
	StatePOS
	StateAddr1
	StateAddr2
	StateType1
	StateType2
	StateData
)

/*
func compareSlices(first []byte, second []byte) bool {

	if len(first) != len(second) {
		return false
	} else {
		for i := 0; i < len(first); i++ {
			if first[i] != second[i] {
				return false
			}
		}
	}
	return true
}
*/

var vkbMessages = map[int]Njoy32MessageDescriptor{
	0x015d: {3 + 11, "GNX-THQ-3 response"},
	0x013d: {3 + 11, "GNX-THQ-1 response"},
	0x010d: {3 + 11, "GNX-THQ-2 response"},

	0x0139: {3 + 12, "FSM-GA response of some sort"},

	0xc80c: {3 + 17, "GNX-THQ response with axes and buttons"},
	0xc809: {3 + 14, "FSM-GA response with encoders"},
	0xc80d: {3 + 18, "FSM-GA response with buttons and encoders"},

	0x9800: {6 + 5, "query to GNX-THQ"},
	0x9831: {6 + 54, "some config message to FSM-GA"},
}

var vkbDevices = map[int]Njoy32DeviceDescriptor{
	// 0x00.000000 00 0
	// 0x01 000000 01 1
	2: {2, 1, "KG12 Twist", ""}, // 0x02 000000 10 2
	3: {3, 1, "KG12", ""},       // 0x03 000000 11 3
	// 0x04 000001 00 0
	5: {5, 1, "TMW Adapter", ""}, // 0x05 000001 01 1
	// 0x06 000001 10 2
	7:  {7, 1, "TRudder", ""},    // 0x07 000001 11 3
	8:  {8, 1, "GF Base", ""},    // 0x08 000010 00 0
	9:  {9, 1, "MCG", ""},        // 0x09 000010 01 1
	10: {10, 1, "GF Base L", ""}, // 0x0a 000010 10 2
	11: {11, 1, "SCG", ""},       // 0x0b 000010 11 3
	12: {12, 1, "SCG-L", ""},     // 0x0c 000011 00 0
	13: {13, 1, "F14 Stick", ""}, // 0x0d 000011 01 1
	// 0x0e 000011 10 2
	15: {15, 1, "MCG Ultimate", ""}, // 0x0f 000011 11 3
	//20: {20,1, "MCG NXT Joker", ""},         // 0x10 000100 00 0
	// 0x11 000100 01 1
	// 0x12 000100 10 2
	// 0x13 000100 11 3
	20: {20, 4, "SEM", ""},                   // 0x14 000101 xx 0,1,2,3
	24: {24, 4, "THQ", "GNX-THQ (3a,4b)"},    // 0x18 000110 xx 0,1,2,3
	28: {28, 1, "STEM", ""},                  // 0x1c 000111 xx 0,1,2,3
	32: {32, 4, "FSM.GA", "FSM-GA (3e,16b)"}, // 0x20 001000 xx 0,1,2,3
	36: {36, 4, "FSM.DIY", ""},               // 0x24 001001 xx 0,1,2,3
	// 0x25 - 0x4f
	80: {80, 1, "MTG-R", ""}, // 0x50 010100 00 0
	81: {81, 1, "MTG-L", ""}, // 0x51 010100 01 1
	82: {82, 1, "MTS-L", ""}, // 0x52 010100 10 2
	83: {83, 1, "MTS-R", ""}, // 0x53 010100 11 3
}

var crc8 = &[]crc.Parameters{ // Common 8-bit CRC Parameters                                          Name 	        	Polynomial	Init	ReflectIn	ReflectOut	FinalXor	Used In / Known As
	{Width: 8, Polynomial: 0x07, Init: 0x00, ReflectIn: false, ReflectOut: false, FinalXor: 0x00}, // CRC-8 	        0x07 		0x00	false		false		0x00		SMBus, CCITT
	{Width: 8, Polynomial: 0x31, Init: 0x00, ReflectIn: true, ReflectOut: true, FinalXor: 0x00},   // CRC-8/MAXIM	    0x31		0x00	true		true		0x00		1-Wire, Dallas, DOW
	{Width: 8, Polynomial: 0x1D, Init: 0xFF, ReflectIn: false, ReflectOut: false, FinalXor: 0xFF}, // CRC-8/SAE-J1850	0x1D		0xFF	false		false		0xFF		Automotive OBD
	{Width: 8, Polynomial: 0x07, Init: 0x00, ReflectIn: false, ReflectOut: false, FinalXor: 0x55}, // CRC-8/ITU	    	0x07		0x00	false		false		0x55		I-432-1
	{Width: 8, Polynomial: 0x07, Init: 0xFF, ReflectIn: true, ReflectOut: true, FinalXor: 0x00},   // CRC-8/ROHC	    0x07		0xFF	true		true		0x00		Robust Header Compression
	{Width: 8, Polynomial: 0x9B, Init: 0x00, ReflectIn: true, ReflectOut: true, FinalXor: 0x00},   // CRC-8/WCDMA	    0x9B		0x00	true		true		0x00		Mobile Communications
	{Width: 8, Polynomial: 0xA7, Init: 0x00, ReflectIn: true, ReflectOut: true, FinalXor: 0x00},   // CRC-8/BLUETOOTH	0xA7		0x00	true		true		0x00		Bluetooth protocols
	{Width: 8, Polynomial: 0x2F, Init: 0xFF, ReflectIn: false, ReflectOut: false, FinalXor: 0xFF}, // CRC-8/AUTOSAR		0x2F		0xFF	false		false		0xFF		Automotive software standard
	{Width: 8, Polynomial: 0x39, Init: 0x00, ReflectIn: true, ReflectOut: true, FinalXor: 0x00},   // CRC-8/DARC	    0x39		0x00	true		true		0x00		Data Radio Channel
	{Width: 8, Polynomial: 0x49, Init: 0x00, ReflectIn: false, ReflectOut: false, FinalXor: 0xFF}, //  CRC-8/GSM-B	    0x49		0x00	false		false		0xFF		GSM Mobile networks
}

var crc16 = []*crc.Parameters{
	crc.X25,
	crc.CCITT,
	crc.CRC16,
	crc.XMODEM,
	crc.XMODEM2,
}

var serialPortMode = &serial.Mode{
	BaudRate: 500000,
	Parity:   serial.NoParity,
	DataBits: 8,
	StopBits: serial.OneStopBit,
}
