// Copyright 2025 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"
	"os"
	"strings"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
)

const (
	formatWarning = "\033[1;97;41m"
	formatReset   = "\033[0m"
)

func main() {
	familyTypes := make(map[string]map[string]bool)

	for _, instanceType := range cloud.KnownInstanceTypes() {
		family := strings.Split(instanceType, ".")[0]

		_, attachmentType := cloud.GetVolumeLimits(instanceType)
		if familyTypes[family] == nil {
			familyTypes[family] = make(map[string]bool)
		}
		familyTypes[family][attachmentType] = true
	}

	loggedWarning := false
	for family, types := range familyTypes {
		if len(types) > 1 {
			if !loggedWarning {
				// Log with red background and bold text to stand out in terminal
				log.Printf("%s!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!%s", formatWarning, formatReset)
				log.Printf("%s!!! WARNING - the following instance types have potentially invalid limits/types !!!%s", formatWarning, formatReset)
				log.Printf("%s!!!          Check if they need to be hardcoded in pkg/volume_limits.go          !!!%s", formatWarning, formatReset)
				log.Printf("%s!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!%s", formatWarning, formatReset)
				loggedWarning = true
			}
			log.Printf("  - Family `%s` has multiple attachment types", family)
		}
	}

	if loggedWarning {
		os.Exit(1)
	}
}
