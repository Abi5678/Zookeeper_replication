package main

import (
	"fmt"
	"time"

	"github.com/go-zookeeper/zk"
)

func main() {
	c, _, err := zk.Connect([]string{"127.0.0.1"}, time.Second) //*10)
	if err != nil {
		panic(err)
	}
	children, stat, ch, err := c.ChildrenW("/")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Children: %+v  Stat: %+v\n", children, stat)
	e := <-ch
	fmt.Printf("e: %+v\n", e)
}
