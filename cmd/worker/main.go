package main

import "log"

func main() {
	log.Println("Worker starting...")
	select {} // block forever
}
