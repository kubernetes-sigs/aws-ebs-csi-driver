package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"
)

type Manifest struct {
	NodeAffinityKey   string
	NodeAffinityValue string
	Volumes           []int
}

func main() {
	// Parse Command-Line args & flags
	nodeAffinityPtr := flag.String("node-affinity", "", "node affinity for pod in form of 'key:value'")
	volumeCountPtr := flag.Int("volume-count", 2, "amount of Volumes to provision")
	flag.Parse()

	nodeAffinityKey, nodeAffinityValue := parseNodeAffinityFlag(nodeAffinityPtr)

	manifest := Manifest{
		NodeAffinityKey:   nodeAffinityKey,
		NodeAffinityValue: nodeAffinityValue,
		Volumes:           make([]int, *volumeCountPtr),
	}

	// Generate manifest to stdout from template file
	var tmplFile = "device_slot_test.tmpl"
	tmpl, err := template.New(tmplFile).ParseFiles(tmplFile)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(os.Stdout, manifest)
	if err != nil {
		panic(err)
	}
}

func parseNodeAffinityFlag(nodeAffinityPtr *string) (string, string) {
	nodeAffinityKey := ""
	nodeAffinityValue := ""
	if len(*nodeAffinityPtr) > 0 {
		nodeAffinity := strings.Split(*nodeAffinityPtr, ":")
		if len(nodeAffinity) != 2 {
			panic(fmt.Errorf("flag '--node-affinity' must take the form 'key:value'"))
		}
		nodeAffinityKey = nodeAffinity[0]
		nodeAffinityValue = nodeAffinity[1]
	}
	return nodeAffinityKey, nodeAffinityValue
}
