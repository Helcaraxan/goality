package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

func main() {
	tmp, err := ioutil.TempFile("", "test")
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	defer tmp.Close()

	for {
		err := russianRoulette()
		if err != nil {
			fmt.Println(err)
			break
		}
	}

	err = russianRoulette()
	if err != nil {
		fmt.Println(err)
	}
}

func russianRoulette() error {
	if time.Now().Second()%2 == 1 {
		return fmt.Errorf("your luck has run out")
	}
	return nil
}
