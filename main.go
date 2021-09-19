package main

import (
	"fmt"
	"github.com/willscott/wikibridge/lib"
)

func main() {
	r, err := lib.GetText("https://wiki.mako.cc", "Center for Extraordinary Postquarantine Dining", "Grocery items")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", r)
}
