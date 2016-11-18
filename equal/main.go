package main

import (
	"fmt"
	"os"

	"github.com/udhos/equalfile"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("usage: equal file1 file2\n")
		os.Exit(2)
	}

	file1 := os.Args[1]
	file2 := os.Args[2]

	equal, err := equalfile.CompareFile(file1, file2)
	if err != nil {
		fmt.Printf("equal: error: %v\n", err)
		os.Exit(3)
	}

	if equal {
		fmt.Println("equal: files match")
		os.Exit(0)
	}

	fmt.Println("equal: files differ")
	os.Exit(1)
}
