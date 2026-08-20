package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	h, err := bcrypt.GenerateFromPassword([]byte("LoadTest-Only-2026!"), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(h))
}
