package main

import "log"

func main() {
	if err := runApplication(); err != nil {
		log.Fatal(err)
	}
}
