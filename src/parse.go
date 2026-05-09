package main

import (
	"fmt"
	"os"
	"time"
)

func displayMessage(packet *Njoy32Message, file *os.File) {
	logEntry := time.Now().Format("15:04:05.000") + " "
	if packet.Response {
		logEntry += "<="
	} else {
		logEntry += "=>"
	}
	logEntry += fmt.Sprintf(" %s-%d ", packet.Device.Model, packet.Device.Index) +
		fmt.Sprintf("(%02d,%02d)", packet.Size.Header, packet.Size.Payload) +
		fmt.Sprintf("{%02X,%02X}", packet.Type.Command, packet.Type.SubCommand)
	if !packet.Known {
		logEntry += "(UNK)"
	}
	if packet.Response {
		logEntry += fmt.Sprintf("[% 2X]", packet.RawData[4:9])
		logEntry += fmt.Sprintf("[% 2X]", packet.RawData[9:11])
	}
	switch packet.Device.Model {
	case "FSM-GA":
		switch packet.Type.Command {
		case 0xc8:
			switch packet.Type.SubCommand {
			case 0xd: // FSM-GA buttons and encoders
				logEntry += fmt.Sprintf("BUTTONS:%08b", packet.RawData[11:15]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[15:17]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[17:19]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[19:21])
			case 0x09: // FSM-GA encoders
				logEntry += fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[13:15]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[15:17])
			default:
				if len(packet.RawData) > 11 {
					logEntry += fmt.Sprintf("[% 2X]", packet.RawData[11:len(packet.RawData)-1])
				}
			}
		case 0x98:
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData[4:len(packet.RawData)-1])
			// 0x00 (5 bytes), 0x31 (54 bytes)
		default:
			if len(packet.RawData) > 11 {
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData[11:len(packet.RawData)-1])
			}
		}
	case "GNX-THQ":
		switch packet.Type.Command {
		case 0xc8:
			switch packet.Type.SubCommand {
			case 0x0c:
				logEntry += fmt.Sprintf("AXIS:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[13:15]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[15:17]) +
					fmt.Sprintf("BUTTONS:%08b", packet.RawData[17:19])
			default:
				if len(packet.RawData) > 11 {
					logEntry += fmt.Sprintf("[% 2X]", packet.RawData[11:len(packet.RawData)-1])
				}
			}
		default:
			if len(packet.RawData) > 11 {
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData[11:len(packet.RawData)-1])
			}
		}
	}
	fmt.Println(logEntry)
	fmt.Fprintln(file, logEntry)
}
