/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
   http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

type Manifest struct {
	NodeAffinityKey   string
	NodeAffinityValue string
	PodName           string
	Volumes           []int
}

func main() {
	// Parse Command-Line args & flags
	nodeAffinityPtr := flag.String("node-affinity", "", "node affinity for pod in form of 'key:value'")
	podNamePtr := flag.String("test-pod-name", "test-pod", "name for pod used in manifest. Default is 'test-pod'")
	volumeCountPtr := flag.Int("volume-count", 2, "amount of Volumes to provision")
	flag.Parse()

	nodeAffinityKey, nodeAffinityValue, err := parseNodeAffinityFlag(nodeAffinityPtr)
	if err != nil {
		log.Fatal(err)
	}

	manifest := Manifest{
		NodeAffinityKey:   nodeAffinityKey,
		NodeAffinityValue: nodeAffinityValue,
		PodName:           *podNamePtr,
		Volumes:           make([]int, *volumeCountPtr),
	}

	// Generate manifest to stdout from template file
	var tmplFile = "device_slot_test.tmpl"
	tmpl, err := template.New(tmplFile).ParseFiles(tmplFile)
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(os.Stdout, manifest)
	if err != nil {
		log.Fatal(err)
	}
}

func parseNodeAffinityFlag(nodeAffinityPtr *string) (string, string, error) {
	nodeAffinityKey := ""
	nodeAffinityValue := ""
	if len(*nodeAffinityPtr) > 0 {
		nodeAffinity := strings.Split(*nodeAffinityPtr, ":")
		if len(nodeAffinity) != 2 {
			return "", "", fmt.Errorf("flag '--node-affinity' must take the form 'key:value'")
		}
		nodeAffinityKey = nodeAffinity[0]
		nodeAffinityValue = nodeAffinity[1]
	}
	return nodeAffinityKey, nodeAffinityValue, nil
}
