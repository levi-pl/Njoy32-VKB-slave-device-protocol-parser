package main

import (
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
						nJoy32RawPacket = append(nJoy32RawPacket, value)
						state = StateSOP
					} else {
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
					//fmt.Println("Known packet " + entry.Name)
				} else {
					packetLength = maxPacketLength
					knownPacket = false
					//fmt.Println("Unknown packet")
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
						state = StateUndefined
					}
				case len(nJoy32RawPacket) > packetLength && knownPacket:
					// packet too long !!! start over
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
		var response Njoy32Message
		if (rawMessage.Data)[1] == syncByte {
			response.Response = false
			deviceByte = 4
		} else {
			response.Response = true
			deviceByte = 1
		}
		response.RawData = (rawMessage.Data)[deviceByte:]
		response.Size.Header = deviceByte
		response.Size.Payload = len((rawMessage.Data)) - deviceByte
		response.Type.Command = int((rawMessage.Data)[deviceByte+2])
		response.Type.SubCommand = int((rawMessage.Data)[deviceByte+3])
		response.Known = rawMessage.Known

		switch (rawMessage.Data)[deviceByte] {
		case 24:
			response.Device = Njoy32Device{"GNX-THQ", 1}
		case 25:
			response.Device = Njoy32Device{"GNX-THQ", 2}
		case 26:
			response.Device = Njoy32Device{"GNX-THQ", 3}
		case 27:
			response.Device = Njoy32Device{"GNX-THQ", 4}
		case 32:
			response.Device = Njoy32Device{"FSM-GA", 1}
		case 33:
			response.Device = Njoy32Device{"FSM-GA", 2}
		case 34:
			response.Device = Njoy32Device{"FSM-GA", 3}
		case 35:
			response.Device = Njoy32Device{"FSM-GA", 4}
		}
		output <- response
	}

}
