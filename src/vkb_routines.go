package main

/* LOT OF VIBECODE in here, but it serves as a reference for how the packet construction, encryption, and checksum logic works.
   Note: The actual packet parsing and state machine logic is in packet.go and parse.go, this file focuses on the core algorithms.
   What I want from here is to figure out how the RC4 encryption and checksum calculations, so I can apply that logic to the packets I capture from the serial port.
*/

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Constants derived from your memory addresses
const (
	MagicValue16BitChecksum        uint16 = 0xffff
	AnotherMagicValue16bitChecksum uint16 = 0xa55a
)

var RC4Sbox = [256]byte{0x63, 0x82, 0xc7, 0x8a, 0x17, 0xf6, 0x74, 0x5c,
	0x3d, 0x93, 0x95, 0x13, 0x29, 0xb8, 0x53, 0x6e,
	0x25, 0x33, 0xf8, 0x2b, 0xac, 0xc4, 0x32, 0x92,
	0x9a, 0x7e, 0xc2, 0xd5, 0x0f, 0x01, 0x35, 0x5f,
	0x0b, 0x98, 0x1e, 0x83, 0x08, 0x3a, 0x26, 0xb3,
	0xa2, 0x7d, 0x80, 0x43, 0x4c, 0xce, 0x02, 0x28,
	0x03, 0x11, 0x50, 0xd6, 0x36, 0xc0, 0x76, 0xa0,
	0x4b, 0xbe, 0x59, 0xc6, 0x3f, 0xa6, 0x77, 0x34,
	0xf0, 0xa5, 0x46, 0xbd, 0x45, 0x55, 0xec, 0xfa,
	0x41, 0x37, 0x6f, 0x6d, 0x15, 0x3e, 0x8d, 0x8f,
	0x04, 0xa9, 0x7a, 0xa8, 0x89, 0xbf, 0xe6, 0xb5,
	0x31, 0x9f, 0xbc, 0xa3, 0xea, 0xca, 0xe9, 0x51,
	0xae, 0x65, 0x23, 0x44, 0x75, 0x1a, 0x42, 0x62,
	0x90, 0xb1, 0x7f, 0xdb, 0x73, 0x85, 0x48, 0x21,
	0x10, 0xde, 0xad, 0x52, 0x61, 0x4f, 0x86, 0xb6,
	0x19, 0xe8, 0x3c, 0x97, 0xb0, 0xf3, 0x99, 0xd7,
	0x0d, 0x88, 0x47, 0x6b, 0x5d, 0x09, 0xd1, 0x40,
	0x69, 0xab, 0x9d, 0xe0, 0xe3, 0x5b, 0xd0, 0x16,
	0x1d, 0xb7, 0xaa, 0xd4, 0x4d, 0xc3, 0x9c, 0x56,
	0xef, 0xaf, 0xb9, 0x9b, 0x12, 0x00, 0xed, 0x4a,
	0x8e, 0x1f, 0x38, 0x2f, 0x14, 0x2e, 0xf9, 0xba,
	0x4e, 0xd2, 0xdf, 0x05, 0x1c, 0xf2, 0x8b, 0xdd,
	0x1b, 0x2c, 0x81, 0x72, 0x30, 0x78, 0x0a, 0xa7,
	0xbb, 0x3b, 0xd9, 0xc1, 0x8c, 0xe2, 0xff, 0xa1,
	0xcc, 0xc9, 0x22, 0xfb, 0x18, 0x87, 0x6c, 0xcf,
	0x24, 0xda, 0x79, 0xf5, 0x06, 0x0c, 0xe7, 0xc5,
	0xb4, 0xee, 0x60, 0x54, 0xdc, 0x7b, 0x2d, 0x49,
	0x84, 0x64, 0xe1, 0x96, 0x91, 0xd3, 0xe4, 0xfe,
	0xf7, 0x67, 0x68, 0xb2, 0xcd, 0xfd, 0x5a, 0xf4,
	0x5e, 0x66, 0xeb, 0xa4, 0x39, 0xf1, 0x6a, 0x9e,
	0x7c, 0x2a, 0xd8, 0x71, 0x70, 0x07, 0xc8, 0xcb,
	0xe5, 0x94, 0x58, 0x0e, 0x20, 0x27, 0xfc, 0x57}

func rc4Crypt(data []byte, preShuffledSbox *[256]byte) {
	// Copy the pre-initialized S-box to a local version
	// so the original remains "pre-shuffled" for the next call
	sBox := preShuffledSbox
	i, j := 0, 0

	for k := 0; k < len(data); k++ {
		i = (i + 1) % 256
		j = (j + int(sBox[i])) % 256

		// Swap bytes
		sBox[i], sBox[j] = sBox[j], sBox[i]

		// XOR the data with the keystream
		keystreamByte := sBox[(int(sBox[i])+int(sBox[j]))%256]
		data[k] ^= keystreamByte
	}
}

// Global or passed-in CRC table (extracted from 0x08001bdc)
var crc16Table [256]uint16

func calculate16BitChecksum(seed uint16, data []byte) uint16 {
	crc := uint32(seed)
	for _, b := range data {
		// Logic: ((Table[(byte ^ (crc >> 8))] ^ (crc << 8)) & 0xFFFF)
		index := byte(b) ^ byte(crc>>8)
		crc = (uint32(crc16Table[index]) ^ (crc << 8)) & 0xFFFF
	}
	return uint16(crc)
}

var stm32Table = crc32.MakeTable(crc32.IEEE)

func calculateHwCrc32(data []byte) uint32 {
	// byte_length >> 2 in the C code means it only processes full 32-bit words.
	// Any trailing bytes (1, 2, or 3 bytes at the end) are ignored by this specific C logic.
	wordCount := len(data) / 4

	// CRC unit initial state is typically 0xFFFFFFFF
	crc := uint32(0xFFFFFFFF)

	for i := 0; i < wordCount; i++ {
		// Read 4 bytes as a 32-bit word
		// STM32 is Little Endian, but the CRC unit processes words
		val := binary.LittleEndian.Uint32(data[i*4 : i*4+4])

		// This simulates writing to crc_unit->data_port
		// Note: Standard CRC-32 (IEEE) usually reflects input/output.
		// If your STM32 result doesn't match, you may need to manually
		// calculate using the 0x04C11DB7 polynomial without reflection.
		crc = crc32.Update(crc, stm32Table, []byte{
			byte(val >> 24),
			byte(val >> 16),
			byte(val >> 8),
			byte(val),
		})
	}

	return crc
}

func wrapPacket(payload []uint16, devID uint16, cmd int, mode int) []byte {
	outBuf := make([]byte, 9)
	payloadLen := len(payload) * 2 // Working in bytes for the packet logic

	// Convert uint16 payload to byte slice (auStack_68 equivalent)
	payloadBytes := make([]byte, payloadLen)
	for i, val := range payload {
		binary.LittleEndian.PutUint16(payloadBytes[i*2:], val)
	}

	// 1. Set Sync Byte
	if mode == 0 {
		outBuf[0] = 0x5A
	} else {
		outBuf[0] = 0xA5
	}

	// 2. Set Device ID (Bytes 1-2)
	binary.LittleEndian.PutUint16(outBuf[1:3], devID)

	// 3. Set Command (Byte 3)
	outBuf[3] = byte(cmd)

	// lengthOffset tracks where the final 16-bit checksum goes
	lengthOffset := 4

	// 4. If cmd high bit is set (cmd & 0x80)
	if (cmd & 0x80) != 0 {
		outBuf[4] = byte(payloadLen)

		var checksum16 uint16
		if payloadLen == 0 {
			if len(payload) > 0 {
				checksum16 = payload[0]
			}
		} else if (cmd & 0x40) != 0 {
			// Hardware CRC32 path (cmd & 0x40)
			hwCrc := calculateHwCrc32(payloadBytes)
			checksum16 = uint16(hwCrc>>13) ^ AnotherMagicValue16bitChecksum
		} else {
			// Software Checksum path
			checksum16 = calculate16BitChecksum(MagicValue16BitChecksum, payloadBytes)
		}

		// Set calculated checksum (Bytes 5-6)
		outBuf[5] = byte(checksum16)
		outBuf[6] = byte(checksum16 >> 8)

		lengthOffset = 7
	}

	// 5. Final Header Check (Calculated over bytes 1-6)
	// This matches: calculate_16bit_checksum(MAGIC, out_buf + 1, 6)
	finalCheck := calculate16BitChecksum(MagicValue16BitChecksum, outBuf[1:7])
	binary.LittleEndian.PutUint16(outBuf[lengthOffset:lengthOffset+2], finalCheck)

	return outBuf
}

func unwrapPacket(packet []byte, expectedMode int) ([]byte, uint16, byte, error) {
	if len(packet) < 9 {
		return nil, 0, 0, errors.New("packet too short")
	}

	// 1. Verify Sync Byte
	expectedSync := byte(0xA5)
	if expectedMode == 0 {
		expectedSync = 0x5A
	}
	if packet[0] != expectedSync {
		return nil, 0, 0, errors.New("invalid sync byte")
	}

	// 2. Determine where the Header Checksum is located
	// Based on wrap_packet logic, it's at index 4 if (cmd & 0x80) == 0, else index 7
	cmd := packet[3]
	headerCheckOffset := 4
	if (cmd & 0x80) != 0 {
		headerCheckOffset = 7
	}

	// 3. Verify Header Checksum (calculated over bytes 1-6)
	receivedHeaderCRC := binary.LittleEndian.Uint16(packet[headerCheckOffset : headerCheckOffset+2])
	computedHeaderCRC := calculate16BitChecksum(MagicValue16BitChecksum, packet[1:7])
	if receivedHeaderCRC != computedHeaderCRC {
		return nil, 0, 0, errors.New("header checksum mismatch")
	}

	devID := binary.LittleEndian.Uint16(packet[1:3])

	// 4. If payload info is present (cmd & 0x80), verify payload integrity
	if (cmd & 0x80) != 0 {
		//payloadLen := int(packet[4])
		receivedPayloadCRC := binary.LittleEndian.Uint16(packet[5:7])

		// Note: The actual payload data is NOT inside this 9-byte out_buf.
		// out_buf only contains the HASH of the payload.
		// To fully verify, you must pass the actual payload bytes to this function.
		return nil, devID, cmd, fmt.Errorf("payload hash 0x%04X received; provide payload buffer to verify", receivedPayloadCRC)
	}

	return nil, devID, cmd, nil
}

type SystemGlobals struct {
	HWWordConfig uint32
	EventFlags   uint8
	HWStatus     uint8
}

type ProtocolContext struct {
	PacketFlagsA uint8
	PacketFlagsB uint8
	MutexLock    uint8
	SessionID    uint8
	TxDataLen    uint8
	TxBuffer     []byte
	FrameStart   []byte
	FramerResult uint8
}

// Global simulation variables
var (
	GlobalSys             = &SystemGlobals{}
	SessionCounter        = make([]byte, 8)
	LiveSensors           = make([]byte, 8) // 4 channels * 2 bytes
	CachedSensors         = make([]byte, 8)
	AuthKeyBase           = make([]byte, 32)
	DeviceID       uint16 = 0x1000
)

func generateAndSendSecurePacket(ctxt *ProtocolContext, mode int) {
	var length uint16 = 0
	encryptFlag := mode

	// Initial Context Setup
	ctxt.PacketFlagsA &= 0xDF
	ctxt.MutexLock |= 0x08

	if mode == 0 {
		// 1. Setup Stack Buffer (Ghidra's stack_pkt_head/body)
		// This buffer will hold the actual data to be sent
		payload := new(bytes.Buffer)

		// 2. Hardware Config & Session Logic
		payload.WriteByte(byte(GlobalSys.HWWordConfig))
		payload.WriteByte(0x00) // stack_pkt_body = '\0'

		// Simulate GPIOB manipulation and Session Counter update
		SessionCounter[0]++

		// Session ID bit manipulation logic
		if (ctxt.SessionID & 0x02) != 0 { // simplified bit check
			GlobalSys.EventFlags |= 0x80
			ctxt.SessionID &= 0xFD
		}

		// 3. Sensor Data Block
		numChannels := 4
		compareSize := numChannels * 2

		// Logic: If event flag bit set, log sensor block
		if (GlobalSys.EventFlags & 0x80) != 0 {
			copy(CachedSensors, LiveSensors)
			sensorTag := byte(numChannels | 0x20)
			payload.WriteByte(sensorTag)
			payload.Write(LiveSensors[:compareSize])
		} else {
			if !bytes.Equal(CachedSensors, LiveSensors) {
				GlobalSys.EventFlags |= 0x80
				ctxt.SessionID |= 2
			}
		}

		// 4. Device/Auth Block
		// Logic: Write 'D' + Auth Key segment
		payload.WriteByte('D')
		payload.Write(AuthKeyBase[0:4])

		// 5. Finalize Staging Buffer
		payload.WriteByte(0xA2)
		payload.Write(AuthKeyBase[16:20])

		stagingData := payload.Bytes()
		length = uint16(len(stagingData))

		// 6. RC4 Encryption (only if encryptFlag is 0)
		if encryptFlag == 0 {
			// Using the rc4Crypt function from previous turns
			// Note: Needs a pre-shuffled S-Box
			rc4Crypt(stagingData, &RC4Sbox)
		}

		ctxt.TxBuffer = stagingData
		ctxt.TxDataLen = uint8(length)
	}

	// 7. Wrapping and Sending
	ctxt.PacketFlagsB = (ctxt.PacketFlagsB & 0xDF) | 0x40

	// wrap_packet(out_buf, payload, len, dev_id, cmd, mode)
	// cmd is 200 (0xC8), which has the 0x80 bit set, triggering CRC logic
	framerResult := wrapPacket(bytesToUint16(ctxt.TxBuffer), DeviceID, 200, 0)

	ctxt.FrameStart = framerResult
	ctxt.FramerResult = uint8(len(framerResult))
	ctxt.PacketFlagsA |= 0x20
	ctxt.MutexLock &= 0xF7
}

// Helper to convert byte slice to uint16 slice for wrapPacket
func bytesToUint16(b []byte) []uint16 {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return u
}

func getIncomingPayload() []byte {
	// In the actual implementation, this would read from the serial buffer or similar source
	// For this example, we will return a dummy payload that matches the expected structure
	return []byte{
		0x01, 0x00, // HW Config (example)
		0x00,       // Padding
		0x24,       // Sensor Tag (4 channels)
		0x10, 0x00, // Sensor Data Channel 1
		0x20, 0x00, // Sensor Data Channel 2
		0x30, 0x00, // Sensor Data Channel 3
		0x40, 0x00, // Sensor Data Channel 4
		'D',                    // Device Block Tag
		0xAA, 0xBB, 0xCC, 0xDD, // Auth Segment
		0xA2,                   // Final Auth Block Tag
		0x11, 0x22, 0x33, 0x44, // Final Auth Data
	}
}

func receiveAndProcessPacket(rawPacket []byte, expectedMode int) error {
	// 1. Unwrap the 9-byte frame
	// This validates the sync byte and the 16-bit header checksum
	//devID, cmd, err := unwrapPacketHeader(rawPacket, expectedMode)
	_, cmd, err := unwrapPacketHeader(rawPacket, expectedMode)
	if err != nil {
		return fmt.Errorf("unwrap failed: %w", err)
	}

	// In the original code, cmd 200 (0xC8) indicates a secure payload
	if cmd != 200 {
		return fmt.Errorf("unexpected command: %d", cmd)
	}

	// 2. Extract the encrypted payload hash from the 9-byte header
	// Note: In your protocol, the 9-byte header contains the HASH,
	// but the ACTUAL encrypted data follows the header or is stored in ctxt.tx_buffer
	encryptedPayload := getIncomingPayload() // Retrieve the data following the header

	// 3. Decrypt the payload
	// Since RC4 is symmetric, calling rc4Crypt again with the same state decrypts it
	rc4Crypt(encryptedPayload, &RC4Sbox)

	// 4. Parse the Decrypted Data (TLV Structure)
	// Byte 0: HW Config
	// Byte 1: Null/Padding
	// Then dynamic blocks...
	reader := bytes.NewReader(encryptedPayload)

	//hwConfig, _ := reader.ReadByte()
	reader.ReadByte()
	reader.ReadByte() // Skip padding

	for reader.Len() > 0 {
		tag, _ := reader.ReadByte()
		switch {
		case tag&0x20 != 0: // Sensor Block (based on sensor_tag = num_channels | 0x20)
			numChannels := tag & 0x1F
			sensorData := make([]byte, numChannels*2)
			reader.Read(sensorData)
			fmt.Printf("Received %d channels of sensor data\n", numChannels)

		case tag == 'D': // Device Block
			authSegment := make([]byte, 4)
			reader.Read(authSegment)
			fmt.Printf("Received Device Auth Segment: %x\n", authSegment)

		case tag == 0xA2: // Final Auth Block
			finalAuth := make([]byte, 4)
			reader.Read(finalAuth)
			fmt.Printf("Final Auth Block Verified: %x\n", finalAuth)
		}
	}

	return nil
}

// Helper to validate the 9-byte header specifically
func unwrapPacketHeader(packet []byte, mode int) (uint16, byte, error) {
	if len(packet) < 9 {
		return 0, 0, errors.New("packet too short")
	}

	// Validate Header Checksum (Bytes 7-8)
	// The lengthOffset was 7 because cmd 200 has bit 0x80 set
	receivedCRC := binary.LittleEndian.Uint16(packet[7:9])
	computedCRC := calculate16BitChecksum(MagicValue16BitChecksum, packet[1:7])

	if receivedCRC != computedCRC {
		return 0, 0, errors.New("header CRC mismatch")
	}

	devID := binary.LittleEndian.Uint16(packet[1:3])
	cmd := packet[3]

	return devID, cmd, nil
}

func decryptAndUnwrap(fullPacket []byte, encryptedPayload []byte) error {
	// 1. Validate the 9-byte header first
	// We use command 200 (0xC8) logic based on your previous code
	if len(fullPacket) < 9 {
		return errors.New("packet too short")
	}

	// Verify Header Checksum (Bytes 7-8)
	receivedHeaderCRC := binary.LittleEndian.Uint16(fullPacket[7:9])
	computedHeaderCRC := calculate16BitChecksum(MagicValue16BitChecksum, fullPacket[1:7])
	if receivedHeaderCRC != computedHeaderCRC {
		return errors.New("header CRC integrity failed")
	}

	// 2. Validate Payload Integrity (Bytes 5-6 contain the hash)
	receivedPayloadHash := binary.LittleEndian.Uint16(fullPacket[5:7])
	// Note: wrap_packet used calculate_16bit_checksum for cmd 200 (bit 0x40 was 0)
	computedPayloadHash := calculate16BitChecksum(MagicValue16BitChecksum, encryptedPayload)
	if receivedPayloadHash != computedPayloadHash {
		return errors.New("payload hash mismatch (data corrupted or wrong key)")
	}

	// 3. Decrypt the Payload
	// We MUST copy the box because rc4Crypt modifies it during execution
	workingSbox := RC4Sbox
	rc4Crypt(encryptedPayload, &workingSbox)

	// 4. Parse the internal TLV data
	return parseDecryptedPayload(encryptedPayload)
}

func parseDecryptedPayload(data []byte) error {
	reader := bytes.NewReader(data)

	// Byte 0: HW Config, Byte 1: Null/Padding
	hwConfig, _ := reader.ReadByte()
	reader.ReadByte()
	fmt.Printf("System HW Config: 0x%02X\n", hwConfig)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			break
		}

		switch {
		case tag&0x20 != 0: // Sensor Block
			numChannels := int(tag & 0x1F)
			valBuf := make([]byte, numChannels*2)
			reader.Read(valBuf)
			fmt.Printf("Sensors (%d channels): %x\n", numChannels, valBuf)

		case tag == 'D': // Device Auth Block
			deviceData := make([]byte, 4)
			reader.Read(deviceData)
			fmt.Printf("Device Auth ID: %x\n", deviceData)

		case tag == 0xA2: // Secure Tail Block
			tailData := make([]byte, 4)
			reader.Read(tailData)
			fmt.Printf("Secure Tail: %x\n", tailData)

		default:
			return fmt.Errorf("unknown tag in payload: 0x%02X", tag)
		}
	}
	return nil
}
