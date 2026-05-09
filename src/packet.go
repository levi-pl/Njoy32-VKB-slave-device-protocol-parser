package main

import (
	"fmt"
	"sync"

	"go.bug.st/serial"
)

type RawPacket struct {
	Data     []byte
	Known    bool
	Extended bool
}

func serialStateMachine(serialPort *serial.Port, readChan chan Njoy32Message, wg *sync.WaitGroup) {
	state := StateUndefined
	packetLength := 0
	extendedPacketLength := false
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
					fmt.Println(colourRed + "[err] (State undefined) Expected start of packet, got " + fmt.Sprintf("%02X", value) + colourReset)
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
					extendedPacketLength = entry.ExtendedSize
					knownPacket = true
				} else {
					packetLength = maxPacketLength
					extendedPacketLength = false
					knownPacket = false
				}
				state = StateData
				nJoy32RawPacket = append(nJoy32RawPacket, value)
			case StateData:
				switch {
				case knownPacket:
					if len(nJoy32RawPacket) == packetLength {
						if value == startOfPacket {
							// We got a complete message, send it to the channel
							parsingQueue <- RawPacket{Data: nJoy32RawPacket, Known: knownPacket, Extended: false}
							// Start collecing new message
							nJoy32RawPacket = nil
							nJoy32RawPacket = append(nJoy32RawPacket, value)
							state = StateSOP
						} else {
							if extendedPacketLength {
								state = StateExtendedData
								nJoy32RawPacket = append(nJoy32RawPacket, value)
							} else {
								fmt.Println(colourRed + "[err] (base data) Expected start of packet, got " + fmt.Sprintf("%02X", value) + colourReset)
								fmt.Println(colourBrightWhite + "[err] Dumping packet data: " + fmt.Sprintf("% 2X", nJoy32RawPacket) + colourReset)
								state = StateUndefined
								nJoy32RawPacket = nil
							}
						}
					} else {
						nJoy32RawPacket = append(nJoy32RawPacket, value)
					}
				default:
					if value == startOfPacket {
						// We got a complete message, send it to the channel
						parsingQueue <- RawPacket{Data: nJoy32RawPacket, Known: knownPacket, Extended: false}
						// Start collecing new message
						nJoy32RawPacket = nil
						nJoy32RawPacket = append(nJoy32RawPacket, value)
						state = StateSOP
						// If the next byte is not the start of packet, there is chance something is wrong
					} else {
						if len(nJoy32RawPacket) < packetLength {
							nJoy32RawPacket = append(nJoy32RawPacket, value)
						} else {
							state = StateUndefined
							nJoy32RawPacket = nil
						}
					}
				}
			case StateExtendedData:
				if value == startOfPacket {
					// We got a complete message, send it to the channel and reset the state
					//fmt.Println(colourBrightYellow + "[info] Received extended packet, length: " + fmt.Sprintf("%d", len(nJoy32RawPacket)) + " base length: " + fmt.Sprintf("%d", packetLength) + colourReset)
					//fmt.Printf(colourBrightYellow+"[info] Extended packet data: % 2X\n"+colourReset, nJoy32RawPacket)
					parsingQueue <- RawPacket{Data: nJoy32RawPacket, Known: knownPacket, Extended: true}
					packetLength = 0
					extendedPacketLength = false
					nJoy32RawPacket = nil
					nJoy32RawPacket = append(nJoy32RawPacket, value)
					state = StateSOP
				} else {
					if len(nJoy32RawPacket) < maxPacketLength {
						nJoy32RawPacket = append(nJoy32RawPacket, value)

					} else {
						fmt.Println(colourRed + "[err] (extended data)Expected start of packet, got " + fmt.Sprintf("%02X", value) + colourReset)
						fmt.Println(colourBrightWhite + "[err] Dumping packet data: " + fmt.Sprintf("% 2X", nJoy32RawPacket) + colourReset)
						state = StateUndefined
						nJoy32RawPacket = nil
					}
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
		processedMessage.Extended = rawMessage.Extended

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
