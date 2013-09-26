// package circ_buf
package main

import (
	"fmt"
	"strconv"
	"log"
)


type CircItem interface{}

// Circ is a circular buffer
type Circ struct {
	Size int
	Items []interface{}
	Start int
	End int
}

func (c *Circ) ReadFromEnd(i int) interface{} {

// ReadFromEnd accepts an int and returns a value
// from Items `i` back from c.End index
func (c *Circ) ReadFromEnd(i int) interface{} {
	// if Size is 10
	return c.Items[(c.End - 1) - (i % c.Size)]
}

func (c *Circ) incrementEnd() {

	if c.End < c.Size - 1 {
		c.End++
		return
	} else if c.End == c.Size - 1 {
		c.End = 0
		return
	}
	log.Fatal("End is greater than max index of Items")
}

func (c *Circ) incrementStart() {
	if c.Start < c.Size - 1 {
		c.Start++
		return
	} else if c.Start == c.Size - 1 {
		c.Start = 0
		return
	}
}

func (c *Circ) increment() {
	c.incrementEnd()
	if c.End == c.Start {
		c.incrementStart()
	}
}
func (c *Circ) AddItem(item interface{}) {
	c.Items[c.End] = interface{}(item)
		c.increment()
}

func main() {

	c := Circ{
		Size: 10,
		Items: make([]interface{}, 10),
	}

	for v := 0; v < c.Size + 5; v++ {
		c.AddItem(strconv.Itoa(v))
		fmt.Println("c.End", c.End)
		fmt.Println("c.Start", c.Start)
	}

	for v := 0; v < 50; v++ {
		fmt.Println("c.ReadBack(v)", c.ReadFromEnd(v))
	}
	fmt.Printf("c: %+v", c)
	fmt.Println("\n")



}