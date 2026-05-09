package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"go.bug.st/serial"
)

func main() {

	selectionBox := tea.NewProgram(initialModel())
	selectedItem, err := selectionBox.Run()
	check(err)

	if len(selectedItem.(model).selected) > 0 {

		vkbSnifferSerialPort, err := serial.Open(selectedItem.(model).selected, serialPortMode)
		check(err)
		defer vkbSnifferSerialPort.Close()
		vkbSnifferSerialPort.ResetInputBuffer()
		vkbSnifferSerialPort.SetReadTimeout(serial.NoTimeout)

		var serialPortRoutines sync.WaitGroup
		serialPacketIn := make(chan Njoy32Message, 10)

		serialPortRoutines.Go(func() { serialStateMachine(&vkbSnifferSerialPort, serialPacketIn, &serialPortRoutines) })

		go func() {
			logFile, err := os.Create("joystick_v2_" + time.Now().Format(time.RFC3339) + ".log")
			check(err)
			defer logFile.Close()
			for msg := range serialPacketIn {
				//if msg.Device.Index == 1 && msg.Type.Command == 0xc8 && msg.Type.SubCommand == 0x0c {
				displayMessage(&msg, logFile)
				//}
			}
		}()
		select {}
	} else {
		fmt.Println("No serial port selected, exiting.")
		os.Exit(1)
	}

}
