package main

import (
	"log"
	"os"

	"github.com/aws/aws-k8s-tester/e2e/tester"
)

func main() {
	test := tester.NewTester("")
	err := test.Start()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
