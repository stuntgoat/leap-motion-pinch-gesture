package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/stuntgoat/leap-motion-pinch-gesture/circbuf"
	"github.com/whoisjake/gomotion"
)


const (
	// number of consecutive values before timeout
	PINCH_DETECT_THRESHOLD = 40

	PINCH_DISTANCE_THRESHOLD = 1800

	// number of pointables to store in a circular buffer
	// for each finger per hand id.
	MAX_POINTABLES_PER_HISTORY = 15

	// number of times that the last distance between 2 points
	// can be greater that the current point when checking convergence
	CONVERGENCE_THRESHOLD = 4

	DEBUG = true
	DEBUG_COLLECT = true
	DEBUG_CONVERGENCE = false
)

var logger *log.Logger

func Debug(msg string) {
	if DEBUG {
		logger.Println(msg)
	}
}

type Point struct {
	X float64
	Y float64
	Z float64
	HandId int
}

func (p *Point) SetFromPointable(pointable *gomotion.Pointable) {
	p.X, p.Y, p.Z = float64(pointable.TipPosition[0]), float64(pointable.TipPosition[1]), float64(pointable.TipPosition[2])
	p.HandId = pointable.HandId
}


// MyPointable holds a circular buffer of
type MyPointable struct {
	History *circbuf.Circ
	lastUpdate time.Time
}

// calculateConvergence takes 2 MyPointable objects and calculates
// the difference between the last 15 points between each of the
// 2 points given.
func calculateConvergence(p1, p2 *MyPointable) bool {
	// get last point data to confirm if the points converging.
	var pointA = Point{}
	var myA interface{}

	var pointB = Point{}
	var myB interface{}

	var dCurrent float64
	var dLast float64

	var failThreshold int

	// we check every other value from the history
	for i := 0; i < MAX_POINTABLES_PER_HISTORY; i += 2 {
		// may be jank
		myA, _ = p1.History.ReadFromStart(i)

		// 2 assert interface is implements Pointable
		myAp, ok := myA.(gomotion.Pointable)
		if ok {
			pointA.SetFromPointable(&myAp)
		}

		myB, _ = p2.History.ReadFromStart(i)

		myBp, ok := myB.(gomotion.Pointable)
		if ok {
			pointA.SetFromPointable(&myBp)
		}

		dCurrent = DistBtwnPoints(&pointA, &pointB)
		if dLast < dCurrent && dLast != 0 {
			failThreshold++
		}
		dLast = dCurrent
	}

	if failThreshold > CONVERGENCE_THRESHOLD {
		if DEBUG_CONVERGENCE {
			Debug(fmt.Sprintf("failThreshold: %d", failThreshold))
		}
		return false
	}
	return true

}


// HandPinchCheck is an object that represents a hand.
// hands can change ids if they disappear and come back into the LeapMotion's
// view. A Hand can have 1 - 5 Pointables. We keep track the last 15 frames
// seen for each pointable. Pointable ids can/will change as pointables
// enter and leave the LeapMotion's view.
type HandPinchCheck struct {
	// unique per hand in each frame
	HandId int

	// listens for pointables
	PointableChan chan gomotion.Pointable

	// sends a pinch event to the listener
	PinchChan chan *Pinch

	// pointable id to last update time
	LastUpdate map[int]time.Time

	// listen if a finger disappeared
	FingerDisappeared chan bool

	// list of pointables per hand and their history
	Pointables map[int]*MyPointable
}

func (hPinchCheck *HandPinchCheck) ListenForPointables() {
	// pointable id to MyPointable object
	var pair []*MyPointable
	var myP *MyPointable
	var ok bool
	var converging bool

	for {
		select {
		case p := <- hPinchCheck.PointableChan:
			hPinchCheck.LastUpdate[p.Id] = time.Now()

			// create this MyPointable object if it doesn't exist
			myP, ok = hPinchCheck.Pointables[p.Id]
			if ok == false {
				cb := circbuf.NewCircBuf(MAX_POINTABLES_PER_HISTORY)
				myP = &MyPointable{
					History: cb,
				}
			}
			hPinchCheck.Pointables[p.Id] = myP
			myP.History.AddItem(p)
			myP.lastUpdate = time.Now()

		case <- hPinchCheck.FingerDisappeared:
			// check if we have only 2 pointables that have been seen
			// very recently
			// check the number of pointables that are less than 5ms old
			// if only 2, and distance between is less than X, send event
			pair = make([]*MyPointable, 0)

			for _, pntbl := range hPinchCheck.Pointables {
				isNew := bool(time.Since(pntbl.lastUpdate) < time.Duration(50 * time.Millisecond))
				// we check it the last update time is recent and
				// if there are at least 8 frames in history
				if isNew && (pntbl.History.Added >= 8) {
					pair = append(pair, pntbl)
					if DEBUG_COLLECT == false {
						continue
					}
				}
				if DEBUG_COLLECT {
					msg := fmt.Sprintf("pointable is new: %t", isNew)
					Debug(msg)
					msg = fmt.Sprintf("pointable had added %d frames", pntbl.History.Added)
					Debug(msg)
				}
			}
			if len(pair) == 2 {
				var lastItem interface{}
				var err error
				args := make([]Point, 0)

				for _, pntbl := range pair {
					point := Point{}

					// last item
					lastItem, err = pntbl.History.ReadFromEnd(0)
					if err != nil {
						logger.Fatal(err.Error)
					}
					goP, ok := lastItem.(gomotion.Pointable)
					if ok {
						point.SetFromPointable(&goP)
						args = append(args, point)
					}
				}
				dist := hPinchCheck.distBetweenPoints(args[0], args[1])
				if dist < PINCH_DISTANCE_THRESHOLD {
					converging = calculateConvergence(pair[0], pair[1])

					// TODO: - send coordinate of pinch
					if converging {
						Debug("PINCH DETECTED")
						goto REMOVE_OLD
					}
					Debug("failed to converge")
				} else  {
					msg := fmt.Sprintf("PINCH_DISTANCE_THRESHOLD: %d\tdistance: %f", PINCH_DISTANCE_THRESHOLD, dist)
					Debug(msg)
				}
			}
			if DEBUG {
				logger.Printf("could not find 2 pointables. Found %d", len(pair))
			}
			goto REMOVE_OLD
		}
	REMOVE_OLD:
		// remove old pointables
		for id, mpt := range hPinchCheck.Pointables {
			if time.Since(mpt.lastUpdate) > time.Duration(60 * time.Millisecond) {
				delete(hPinchCheck.Pointables, id)
			}
		}
	}
}

func (hPinchCheck *HandPinchCheck) distBetweenPoints(a, b Point) float64 {
	return DistanceBetween(a.X, a.Y, a.Z, b.X, b.Y, b.Z)
}

func DistBtwnPoints(p1, p2 *Point) float64 {
	return DistanceBetween(p1.X, p1.Y, p1.Z, p2.X, p2.Y, p2.Z)
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
		// clear()

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
					LastUpdate: make(map[int]time.Time),
					Pointables: make(map[int]*MyPointable),
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


func init() {
	logger = log.New(os.Stderr, "[leap pinch] ", log.LstdFlags|log.Lshortfile)
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
