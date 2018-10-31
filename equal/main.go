package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"

	"github.com/udhos/equalfile"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("usage: equal file1 file2 [...fileN]\n")
		os.Exit(2)
	}

	if compareFiles(os.Args[1:]) {
		fmt.Println("equal: files match")
		return // cleaner than os.Exit(0)
	}

	fmt.Println("equal: files differ")
	os.Exit(1)
}

func compareFiles(files []string) bool {

	options := equalfile.Options{}

	if str := os.Getenv("DEBUG"); str != "" {
		options.Debug = true
	}

	fmt.Printf("Debug=%v DEBUG=[%s]\n", options.Debug, os.Getenv("DEBUG"))

	if str := os.Getenv("FORCE_FILE_READ"); str != "" {
		options.ForceFileRead = true
	}

	if str := os.Getenv("MAX_SIZE"); str != "" {
		var errConv error
		options.MaxSize, errConv = strconv.ParseInt(str, 10, 64)
		if errConv != nil {
			fmt.Printf("Failure parsing MAX_SIZE=[%s]: %v\n", os.Getenv("MAX_SIZE"), errConv)
		}
	}

	var bufSize int64
	if str := os.Getenv("BUF_SIZE"); str != "" {
		var errConv error
		bufSize, errConv = strconv.ParseInt(str, 10, 64)
		if errConv != nil {
			fmt.Printf("Failure parsing BUF_SIZE=[%s]: %v\n", os.Getenv("BUF_SIZE"), errConv)
		}
	}

	var noHash bool
	if str := os.Getenv("NO_HASH"); str != "" {
		noHash = true
	}

	var compareOnMatch bool
	if str := os.Getenv("COMPARE_ON_MATCH"); str != "" {
		compareOnMatch = true
	}

	if options.Debug {
		fmt.Printf("ForceFileRead=%v FORCE_FILE_READ=[%s]\n", options.ForceFileRead, os.Getenv("FORCE_FILE_READ"))
		fmt.Printf("MaxSize=%d MAX_SIZE=[%s]\n", options.MaxSize, os.Getenv("MAX_SIZE"))
		fmt.Printf("bufSize=%d BUF_SIZE=[%s]\n", bufSize, os.Getenv("BUF_SIZE"))
		fmt.Printf("noHash=%v NO_HASH=[%s]\n", noHash, os.Getenv("NO_HASH"))
		fmt.Printf("compareOnMatch=%v COMPARE_ON_MATCH=[%s]\n", compareOnMatch, os.Getenv("COMPARE_ON_MATCH"))
	}

	var buf []byte
	if bufSize > 0 {
		buf = make([]byte, bufSize)
	}

	var cmp *equalfile.Cmp

	if len(files) > 2 && !noHash {
		cmp = equalfile.NewMultiple(buf, options, sha256.New(), compareOnMatch)
	} else {
		cmp = equalfile.New(buf, options)
	}

	match := true

	for i := 0; i < len(files)-1; i++ {
		p0 := files[i]
		for _, p := range files[i+1:] {
			equal, err := cmp.CompareFile(p0, p)
			if err != nil {
				fmt.Printf("equal(%s,%s): error: %v\n", p0, p, err)
				match = false
				continue
			}
			if !equal {
				if options.Debug {
					fmt.Printf("equal(%s,%s): files differ\n", p0, p)
				}
				match = false
				continue
			}
			if options.Debug {
				fmt.Printf("equal(%s,%s): files match\n", p0, p)
			}
		}
	}

	return match
}
