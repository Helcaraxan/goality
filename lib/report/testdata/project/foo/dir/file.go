package dir

import "fmt"

func LetItSnow() {
	myString := "Jingle bells"
	for i := 0; i < 5; i++ {
		myString := "Let it snow!"
		fmt.Println(myString)
	}
	fmt.Println(myString)
}

func unworthy() {}
