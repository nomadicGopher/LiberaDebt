package main

import (
	"github.com/outrigdev/outrig"
	"log"
)

func main() {
	outrig.Init("LiberaDebt", nil)
	defer outrig.AppDone()

	log.Println("Hello world!")
}
