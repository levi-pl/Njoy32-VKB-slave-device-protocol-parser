package main

import (
	"fmt"
	"sync"

	"go.bug.st/serial"
)

type RawPacket struct {
	Data  []byte
	Known bool
}

func serialStateMachine(serialPort *serial.Port, readChan chan Njoy32Message, wg sync.WaitGroup) {
	state := StateUndefined
	packetLength := 0
	knownPacket := false
	serialPortBuffer := make([]byte, 128)
	nJoy32RawPacket := make([]byte, 0, maxPacketLength)
	parsingQueue := make(chan RawPacket, 10)
	wg.Go(func() { processPacket(parsingQueue, readChan) })
	for {
		numberOfBytes, err := (*serialPort).Read(serialPortBuffer)
		check(err)

		for _, value := range serialPortBuffer[:numberOfBytes] {
			switch state {
			case StateUndefined:
				if value == startOfPacket {
					nJoy32RawPacket = nil
					nJoy32RawPacket = append(nJoy32RawPacket, value)
					state = StateSOP
				} else {
					fmt.Println(colourRed + "[err] Expected start of packet, got " + fmt.Sprintf("%02X", value) + colourReset)
				}
			case StateSOP:
				if value == syncByte {
					state = StateFirstSync
				} else {
					state = StateAddr1
				}
				nJoy32RawPacket = append(nJoy32RawPacket, value)
			case StateFirstSync:
				if value == syncByte {
					nJoy32RawPacket = append(nJoy32RawPacket, value)
					state = StateSecondSync
				} else {
					nJoy32RawPacket = nil
					if value == startOfPacket {
						fmt.Println(colourRed + "[err] Expected sync byte, got start of packet, Starting new packet" + colourReset)
						nJoy32RawPacket = append(nJoy32RawPacket, value)
						state = StateSOP
					} else {
						fmt.Println(colourRed + "[err] Expected sync byte, got " + fmt.Sprintf("%02X", value) + colourReset)
						state = StateUndefined
					}
				}
			case StateSecondSync:
				if value == endPreamble {
					nJoy32RawPacket = append(nJoy32RawPacket, value)
					state = StatePOS
				} else {
					nJoy32RawPacket = nil
					if value == startOfPacket {
						nJoy32RawPacket = append(nJoy32RawPacket, value)
						state = StateSOP
					} else {
						state = StateUndefined
					}
				}
			case StatePOS:
				nJoy32RawPacket = append(nJoy32RawPacket, value)
				state = StateAddr1
			case StateAddr1:
				nJoy32RawPacket = append(nJoy32RawPacket, value)
				state = StateAddr2
			case StateAddr2:
				nJoy32RawPacket = append(nJoy32RawPacket, value)
				state = StateType1
			case StateType1:
				nJoy32RawPacket = append(nJoy32RawPacket, value)
				state = StateType2
			case StateType2:
				calculatedMessageType := int(nJoy32RawPacket[len(nJoy32RawPacket)-2])*0x100 + int(nJoy32RawPacket[len(nJoy32RawPacket)-1])
				entry, ok := vkbMessages[calculatedMessageType]
				if ok {
					packetLength = entry.Size
					knownPacket = true
				} else {
					packetLength = maxPacketLength
					knownPacket = false
				}
				state = StateData
				nJoy32RawPacket = append(nJoy32RawPacket, value)
			case StateData:
				switch {
				case !knownPacket && len(nJoy32RawPacket) <= packetLength:
					if value == startOfPacket {
						// We got a complete message but we don't know the length, so we assume it's the start of a new message
						parsingQueue <- RawPacket{Data: nJoy32RawPacket, Known: knownPacket}
						packetLength = 0
						nJoy32RawPacket = nil
						state = StateSOP
					}
					if len(nJoy32RawPacket) < packetLength {
						nJoy32RawPacket = append(nJoy32RawPacket, value)
					} else {
						packetLength = 0
						nJoy32RawPacket = nil
						state = StateUndefined
					}
				case len(nJoy32RawPacket) == packetLength:
					// We got a complete message, send it to the channel and reset the state
					parsingQueue <- RawPacket{Data: nJoy32RawPacket, Known: knownPacket}
					packetLength = 0
					nJoy32RawPacket = nil
					// Start collecing new message. If the next byte is not the start of packet, there is chance something is wrong
					if value == startOfPacket {
						nJoy32RawPacket = append(nJoy32RawPacket, value)
						state = StateSOP
					} else {
						fmt.Println(colourRed + "[err] Expected start of packet, got " + fmt.Sprintf("%02X", value) + colourReset)
						state = StateUndefined
					}
				case len(nJoy32RawPacket) > packetLength && knownPacket:
					// packet too long !!! start over
					fmt.Println(colourRed + "[err] Packet too long, expected " + fmt.Sprintf("%d", packetLength) + " got " + fmt.Sprintf("%02X", len(nJoy32RawPacket)) + colourReset)
					packetLength = 0
					nJoy32RawPacket = nil
					state = StateUndefined
				default:
					nJoy32RawPacket = append(nJoy32RawPacket, value)
				}
			}
		}
	}
}

func processPacket(queue chan RawPacket, output chan Njoy32Message) {
	//DISPLAYS whole PACKET including header
	//fmt.Printf("DEBUG [% 2X](%d)\n", (*packet), len(*packet))
	//calculateVariousChecksums((*packet))
	for rawMessage := range queue {
		var deviceByte int
		var processedMessage Njoy32Message
		if (rawMessage.Data)[1] == syncByte {
			processedMessage.Response = false
			deviceByte = 4
		} else {
			processedMessage.Response = true
			deviceByte = 1
		}
		processedMessage.RawData = (rawMessage.Data)[deviceByte+4:]
		processedMessage.Size.Header = deviceByte + 4
		processedMessage.Size.Payload = len((rawMessage.Data)) - processedMessage.Size.Header
		processedMessage.Type.Command = (rawMessage.Data)[deviceByte+2]
		processedMessage.Type.SubCommand = (rawMessage.Data)[deviceByte+3]
		processedMessage.Known = rawMessage.Known

		switch (rawMessage.Data)[deviceByte] {
		case 24:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 24, 1}
		case 25:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 24, 2}
		case 26:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 24, 3}
		case 27:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 24, 4}
		case 32:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 32, 1}
		case 33:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 32, 2}
		case 34:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 32, 3}
		case 35:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], 32, 4}
		default:
			processedMessage.Device = Njoy32Device{(rawMessage.Data)[deviceByte+1], (rawMessage.Data)[deviceByte], 0}
		}
		output <- processedMessage
	}

}
