package main

import (
	"fmt"
	"os"
	"time"
)

func displayMessage(packet *Njoy32Message, file *os.File) {
	baseLength := vkbMessages[int(packet.Type.Command)*0x100+int(packet.Type.SubCommand)].Size - 5
	if !packet.Response {
		baseLength -= 3
	}
	logEntry := time.Now().Format("15:04:05.000") + " "

	if packet.Response {
		logEntry += colourBrightYellow + "<=" + colourReset
	} else {
		logEntry += colourCyan + "=>" + colourReset
	}
	logEntry += colourBold + fmt.Sprintf(" %s-%d ", vkbDevices[int(packet.Device.ModelID)].Model, packet.Device.Index) + colourBrightBlue +
		fmt.Sprintf("(%02d,%02d)", packet.Size.Header, baseLength) + colourBrightYellow +
		fmt.Sprintf("{%02X,%02X}", packet.Type.Command, packet.Type.SubCommand) + colourReset

	if !packet.Known {
		logEntry += colourBrightWhite + "(UNK)" + colourReset + fmt.Sprintf("[% 2X]", packet.RawData)
	} else {
		logEntry += colourBrightGreen + "(" + vkbMessages[int(packet.Type.Command)*0x100+int(packet.Type.SubCommand)].Name + ")" + colourReset
		if packet.Extended && len(packet.RawData) > vkbMessages[int(packet.Type.Command)*0x100+int(packet.Type.SubCommand)].Size {
			logEntry += colourBrightRed + "(EXT)" + colourReset
		}

		switch packet.Type.Command {
		case 0x01: // THQ reports
			switch packet.Type.SubCommand {
			case 0x0d, 0x3d, 0x5d:
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData[:11])
				if packet.Extended && len(packet.RawData) > baseLength {
					logEntry += colourBrightWhite + fmt.Sprintf("EXTENDED:[% 2X]", packet.RawData[11:]) + colourReset
					// Extended data can be present in some cases, for example when the device is in a special mode, but its content is currently unknown
					// It seems to be present in some THQ reports as well, but it's not clear yet how to correlate it with specific subcommands or conditions
					// It can be up to 5 bytes long, but the actual length and structure of this extended data is still unknown and may require further investigation
				}
			default:
				logEntry += "(unknown " + fmt.Sprintf("%02X", packet.Type.SubCommand) + " subcommand)" + fmt.Sprintf("[% 2X]", packet.RawData)
			}
		case 0xc8: // reports
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData[:5])
			logEntry += fmt.Sprintf("[% 2X]", packet.RawData[5:7])
			switch packet.Type.SubCommand {
			case 0xd: // FSM-GA buttons and encoders
				logEntry += fmt.Sprintf("BUTTONS:%08b", packet.RawData[7:11]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[13:15]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[15:17])
				if packet.Extended && len(packet.RawData) > baseLength {
					logEntry += colourBrightWhite + fmt.Sprintf("EXTENDED:[% 2X]", packet.RawData[17:]) + colourReset
					// Extended data can be present in some cases, for example when the device is in a special mode, but its content is currently unknown
					// It seems to be present in some THQ reports as well, but it's not clear yet how to correlate it with specific subcommands or conditions
					// It can be up to 5 bytes long, but the actual length and structure of this extended data is still unknown and may require further investigation
				}
			case 0x09: // FSM-GA encoders
				logEntry += fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[7:9]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[9:11]) +
					fmt.Sprintf("ENCODER:[% 2X]", packet.RawData[11:13])
				if packet.Extended && len(packet.RawData) > baseLength {
					logEntry += colourBrightWhite + fmt.Sprintf("EXTENDED:[% 2X]", packet.RawData[13:]) + colourReset
					// Extended data can be present in some cases, for example when the device is in a special mode, but its content is currently unknown
					// It seems to be present in some THQ reports as well, but it's not clear yet how to correlate it with specific subcommands or conditions
					// It can be up to 5 bytes long, but the actual length and structure of this extended data is still unknown and may require further investigation
				}
			case 0x0c: // THQ axis and buttons
				packet.RawData[10] = packet.RawData[10] ^ 0x45 // some kind of simple obfuscation, not sure yet if it's always the same or if it can vary, but it seems to be present in all FSM-GA reports
				logEntry += fmt.Sprintf("AXIS:[% 2X]", packet.RawData[7:9]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[9:11]) +
					fmt.Sprintf("AXIS:[% 2X]", packet.RawData[11:13]) +
					fmt.Sprintf("BUTTONS:%08b", packet.RawData[13:15])
				if packet.Extended && len(packet.RawData) > baseLength {
					logEntry += colourBrightWhite + fmt.Sprintf("EXTENDED:[% 2X]", packet.RawData[15:]) + colourReset
					// Extended data can be present in some cases, for example when the device is in a special mode, but its content is currently unknown
					// It seems to be present in some THQ reports as well, but it's not clear yet how to correlate it with specific subcommands or conditions
					// It can be up to 5 bytes long, but the actual length and structure of this extended data is still unknown and may require further investigation
				}
			default:
				logEntry += "(unknown " + fmt.Sprintf("%02X", packet.Type.SubCommand) + " subcommand)" + fmt.Sprintf("[% 2X]", packet.RawData[:baseLength])
				if packet.Extended && len(packet.RawData) > baseLength {
					logEntry += colourBrightWhite + fmt.Sprintf("EXTENDED:[% 2X]", packet.RawData[baseLength:]) + colourReset
					// Extended data can be present in some cases, for example when the device is in a special mode, but its content is currently unknown
					// It seems to be present in some THQ reports as well, but it's not clear yet how to correlate it with specific subcommands or conditions
					// It can be up to 5 bytes long, but the actual length and structure of this extended data is still unknown and may require further investigation
				}
			}
		case 0x98: // requests
			switch packet.Type.SubCommand {
			default:
				logEntry += fmt.Sprintf("[% 2X]", packet.RawData)
			}
		default:
			logEntry += "(unknown command group " + fmt.Sprintf("%02X", packet.Type.Command) + ")" + fmt.Sprintf("[% 2X]", packet.RawData)
		}
	}

	fmt.Println(logEntry)
	fmt.Fprintln(file, logEntry)
}
