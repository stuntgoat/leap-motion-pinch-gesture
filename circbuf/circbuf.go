package circbuf

import (
	"errors"
	"log"
)


func NewCircBuf(size int) (circ *Circ) {
	circ = &Circ{
		Size: size,
		Items: make([]interface{}, size),
	}
	return
}

// Circ is a circular buffer
type Circ struct {
	Size int
	Items []interface{}
	Start int
	End int
	atLeast int
	Added int64
}

// ReadFromEnd accepts an int and returns a value
// from Items, `i` back from c.End index
func (c *Circ) ReadFromEnd(i int) (item interface{}, err error) {
	pIndex := (c.Size - (i % c.Size) + c.End - 1) % c.Size

	if pIndex > c.atLeast {
		return nil, errors.New("end of items")
	}
	return c.Items[pIndex], nil
}

func (c *Circ) GetLastItem() (item interface{}) {
	item, _ = c.ReadFromEnd(0)
	return item
}

func (c *Circ) ReadFromStart(i int) (item interface{}, err error) {
	var pIndex int
	if i < c.Size {
		pIndex = i
	} else {
		pIndex = (i + c.Start - 1) % c.Size
	}

	if pIndex >= c.atLeast {
		return nil, errors.New("end of items")
	}
	return c.Items[pIndex], nil
}

func (c *Circ) incrementEnd() {
	if c.End < c.Size - 1 {
		c.End++
		return
	} else if c.End == c.Size - 1 {
		c.End = 0
		return
	}
}

func (c *Circ) incrementStart() {
	if c.Start < c.Size - 1 {
		c.Start++
		return
	} else if c.Start == c.Size - 1 {
		c.Start = 0
		return
	}
	log.Fatal("Start is greater than max index of Items")
}

func (c *Circ) increment() {

	c.incrementEnd()
	if c.End == c.Start {
		c.incrementStart()
	}
	if c.atLeast != c.Size {
		c.atLeast++
	}
}

func (c *Circ) AddItem(item interface{}) {
	c.Items[c.End] = interface{}(item)
	c.increment()
	c.Added++
}
