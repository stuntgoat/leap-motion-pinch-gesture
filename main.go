package main

import (
	"fmt"
	"math"
	"runtime"
	"time"

	"github.com/whoisjake/gomotion"
)


var (
// TODO: - check consecutive distance
	PINCH_DETECT_THRESHOLD = 40 // number of consecutive values before timeout
	PINCH_DISTANCE_THRESHOLD = 950.0
)

type Point struct {
	X float64
	Y float64
	Z float64
}

func (p *Point) SetFromPointable(pointable *gomotion.Pointable) {
	p.X, p.Y, p.Z = float64(pointable.TipPosition[0]), float64(pointable.TipPosition[1]), float64(pointable.TipPosition[2])
}

// HandPinchCheck maintains 'pinch' state per 2 Pointables
// per Hand.
// listen on a Pinching channel to detect pinch events;
// send the event on a Pinch channel.

type PointAble struct {
	pointable *gomotion.Pointable
	lastUpdate time.Time
}


type HandPinchCheck struct {
	HandId int

	PointableChan chan gomotion.Pointable // listens for pointables

	PinchChan chan *Pinch // sends a pinch event

	lastUpdate map[int]time.Time // pointable id to last update time

	FingerDisappeared chan bool // listen if a finger disappeared

	Pointables map[int]PointAble
}

func (hPinchCheck *HandPinchCheck) ListenForPointables() {
	var now time.Time

	for  {
		select {
		case p := <- hPinchCheck.PointableChan:
			now = time.Now()
			hPinchCheck.lastUpdate[p.Id] = now
			hPinchCheck.Pointables[p.Id] = PointAble{&p, now}

		case <- hPinchCheck.FingerDisappeared:
			// check if we have only 2 pointables that have been seen
			// very recently
			// check the number of pointables that are less than 5ms old
			// if only 2, and distance between is less than X, send event

			pair := map[int]*PointAble{}

			for hid, pntbl := range hPinchCheck.Pointables {
				isNew := bool(time.Since(pntbl.lastUpdate) < time.Duration(100 * time.Millisecond))
				if isNew {
					pair[hid] = &pntbl
				}
			}

			if len(pair) == 2 {
				args := make([]Point, 2)
				for _, pntbl := range pair {
					point := Point{}

					point.SetFromPointable(pntbl.pointable)
					args = append(args, point)
				}
				dist := hPinchCheck.distBetweenPoints(args[0], args[1])
				if dist <= PINCH_DISTANCE_THRESHOLD {
					// TODO: - send coordinate of pinch
					fmt.Println("PINCH DETECTED")
					fmt.Println("***************************************")
					fmt.Println("***************************************")
					fmt.Println("***************************************")
				} else  {
					fmt.Println("dist", dist, "PINCH_DISTANCE_THRESHOLD", PINCH_DISTANCE_THRESHOLD)

				}
			} else {
				fmt.Println("pair is not length 2")
				fmt.Println("len(pair)", len(pair))
				fmt.Println("len(pair)", len(pair))
				fmt.Println("len(pair)", len(pair))
			}

		}

		// remove old pointables
		for id, t := range hPinchCheck.lastUpdate {
			if time.Since(t) > time.Duration(200*time.Millisecond) {
				// fmt.Println("deleting", id)
				delete(hPinchCheck.lastUpdate, id)
			}
		}
	}
}

func (hPinchCheck *HandPinchCheck) distBetweenPoints(a, b Point) float64 {
	return DistanceBetween(a.X, a.Y, a.Z, b.X, b.Y, b.Z)
}

// var HandOnePinch = make(chan gomotion.Pointable, PINCH_DETECT_THRESHOLD * 2)
// var HandTwoPinch = make(chan gomotion.Pointable, PINCH_DETECT_THRESHOLD * 2)

// send all pointables to this router and
type HandPinchRouter struct {
	FrameChan chan *gomotion.Frame
	PinchChecks map[int]HandPinchCheck // map of hand ids to
	// TODO: get the pinch events out
}


type PPH struct {
	HandId int
	NumPointables int
	lastUpdate time.Time
}

func clear() {
	fmt.Println("\033[2J\033[H")
}

// RouteHands will route a Pointable to the respective hand pinch
// channel.
func (hPinchRouter *HandPinchRouter) RouteHand() {
	// TODO: remove old values in maps to prevent memory leak

	// check pointables per hand
	var currPerHand = map[int]*PPH{}
	var pastPerHand = map[int]*PPH{}
	var handId int
	for frame := range hPinchRouter.FrameChan {
		// need to keep track of the number of fingers that are here
		// and the number that were before this to register
		// when a pinch event occurs on a hand

		// for debugging
		clear()

		for _, pointable := range frame.Pointables {
			handId = pointable.HandId
			if handId == -1 {
				continue
			}

			// check the current hand id pointables
			if pph, ok := currPerHand[handId]; ok {
				pph.NumPointables += 1
			} else {
				pph = &PPH{
					HandId: handId,
					NumPointables: 0,
				}
				pph.NumPointables++
				currPerHand[handId] = pph
			}

			pc, ok := hPinchRouter.PinchChecks[handId];
			if ok {
				pc.PointableChan <- pointable
			} else {
				hpc := HandPinchCheck{
					HandId: pointable.HandId,
					PointableChan: make(chan gomotion.Pointable),
					lastUpdate: make(map[int]time.Time),
					Pointables: make(map[int]PointAble),
					FingerDisappeared: make(chan bool),
				}
				go hpc.ListenForPointables()
				hpc.PointableChan <- pointable
				hPinchRouter.PinchChecks[handId] = hpc
			}
		}
		// calculate the current pointables per hand and see if any disappeared,
		// if so, send the FingerDisappeared to the hand's FingerDisappeared chan.
		for pHandId, pph := range pastPerHand {
			if cHnum, ok := currPerHand[pHandId]; ok {
				// race condition!!!
				if cHnum.NumPointables < pph.NumPointables {
					// pHandId had 1 or more fingers disappear:
					// FingerDisappeared!!!
					hPinchRouter.PinchChecks[pHandId].FingerDisappeared <- true;
				}
			} else {
				// pHandId had all fingers disappear: FingerDisappeared!!!
				hPinchRouter.PinchChecks[pHandId].FingerDisappeared <- true;
			}
		}
		pastPerHand = currPerHand
		currPerHand = map[int]*PPH{}
	}
}

type Pinch struct {
	Point
	HandID int
}


// DistanceBetween takes the distance between 2 3D points and
// caluclates the total distance between them by calculating the
// sum of the squares of the difference of each x, y and z coordinate.
func DistanceBetween(x, y, z, x2, y2, z2 float64) float64 {
	xSqDiff := math.Pow(x - x2, 2)
	ySqDiff := math.Pow(y - y2, 2)
	zSqDiff := math.Pow(z - z2, 2)
	return xSqDiff + ySqDiff + zSqDiff
}


func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	device, err := gomotion.GetDevice("ws://127.0.0.1:6437/v3.json")
	if err != nil {
		panic(err.Error())
	}
    defer device.Close()
    device.Listen()

	var router = HandPinchRouter{
		FrameChan: make(chan *gomotion.Frame),
		PinchChecks: make(map[int]HandPinchCheck),
	}

	go router.RouteHand()
	// send frames to frames channel in a goroutine

	for frame := range device.Pipe {
		router.FrameChan <- frame
	}
}
