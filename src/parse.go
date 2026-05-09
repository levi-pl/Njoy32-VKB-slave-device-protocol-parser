package main

import (
	"fmt"
	"os"
	"time"
)

func displayMessage(packet *Njoy32Message, file *os.File) {
	logEntry := time.Now().Format("15:04:05.000") + " "
	if packet.Response {
		logEntry += colourBrightYellow + "<=" + colourReset
	} else {
		logEntry += colourCyan + "=>" + colourReset
	}
	logEntry += colourBold + fmt.Sprintf(" %s-%d ", vkbDevices[int(packet.Device.ModelID)].Model, packet.Device.Index) + colourBrightBlue +
		fmt.Sprintf("(%02d,%02d)", packet.Size.Header, packet.Size.Payload) + colourBrightYellow +
		fmt.Sprintf("{%02X,%02X}", packet.Type.Command, packet.Type.SubCommand) + colourReset

	if !packet.Known {
		logEntry += colourBrightWhite + "(UNK)" + colourReset + fmt.Sprintf("[% 2X]", packet.RawData)
	} else {
		logEntry += colourBrightGreen + "(" + vkbMessages[int(packet.Type.Command)*0x100+int(packet.Type.SubCommand)].Name + ")" + colourReset
		//switch packet.Device.Model {
		//case "FSM-GA":
		switch packet.Type.Command {
		case 0xc8: // reports
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData[:5])
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData[5:7])
			switch packet.Type.SubCommand {
			case 0xd: // FSM-GA buttons and encoders
				logEntry += fmt.Sprintf("BUTTONS:%08b", packet.RawData[7:11]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[13:15]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[15:17])
			case 0x09: // FSM-GA encoders
				logEntry += fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[7:9]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[9:11]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[11:13])
			case 0x0c: // THQ axis and buttons
				logEntry += fmt.Sprintf("AXIS:[% 2X]", packet.RawData[7:9]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[9:11]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("BUTTONS:%08b", packet.RawData[13:15])
			default:
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData)
			}
		case 0x98: // requests
			switch packet.Type.SubCommand {
			default:
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData)
			}
		default:
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData)
		}
		//}
	}

	fmt.Println(logEntry)
	fmt.Fprintln(file, logEntry)
}
