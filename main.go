package main

import (
	"fmt"
	"runtime"

	"github.com/stuntgoat/pinch"
	"github.com/whoisjake/gomotion"
)


func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	device, err := gomotion.GetDevice("ws://127.0.0.1:6437/v3.json")
	if err != nil {
		panic(err.Error())
	}
    defer device.Close()

    device.Listen()

	var router = pinch.HandPinchRouter{
		FrameChan: make(chan *gomotion.Frame),
		PinchChecks: make(map[int]pinch.HandPinchCheck),
		PinchChan: make(chan *pinch.Pinch),
	}

	go router.RouteHand()
	for {
		select {
		case frame := <- device.Pipe:
			router.FrameChan <- frame

		case pinch := <- router.PinchChan:
			fmt.Printf("PINCH DETECTED: %+v\n", pinch)
		}
	}
}