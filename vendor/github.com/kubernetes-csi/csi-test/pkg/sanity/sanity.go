/*
Copyright 2017 Luis Pabón luis@portworx.com

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

package sanity

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/utils"
	yaml "gopkg.in/yaml.v2"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// CSISecrets consists of secrets used in CSI credentials.
type CSISecrets struct {
	CreateVolumeSecret              map[string]string `yaml:"CreateVolumeSecret"`
	DeleteVolumeSecret              map[string]string `yaml:"DeleteVolumeSecret"`
	ControllerPublishVolumeSecret   map[string]string `yaml:"ControllerPublishVolumeSecret"`
	ControllerUnpublishVolumeSecret map[string]string `yaml:"ControllerUnpublishVolumeSecret"`
	NodeStageVolumeSecret           map[string]string `yaml:"NodeStageVolumeSecret"`
	NodePublishVolumeSecret         map[string]string `yaml:"NodePublishVolumeSecret"`
	CreateSnapshotSecret            map[string]string `yaml:"CreateSnapshotSecret"`
	DeleteSnapshotSecret            map[string]string `yaml:"DeleteSnapshotSecret"`
}

// Config provides the configuration for the sanity tests. It
// needs to be initialized by the user of the sanity package.
type Config struct {
	TargetPath     string
	StagingPath    string
	Address        string
	SecretsFile    string
	TestVolumeSize int64
}

// SanityContext holds the variables that each test can depend on. It
// gets initialized before each test block runs.
type SanityContext struct {
	Config  *Config
	Conn    *grpc.ClientConn
	Secrets *CSISecrets
}

// Test will test the CSI driver at the specified address by
// setting up a Ginkgo suite and running it.
func Test(t *testing.T, reqConfig *Config) {
	sc := &SanityContext{
		Config: reqConfig,
	}

	registerTestsInGinkgo(sc)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSI Driver Test Suite")
}

func GinkgoTest(reqConfig *Config) {
	sc := &SanityContext{
		Config: reqConfig,
	}

	registerTestsInGinkgo(sc)
}

func (sc *SanityContext) setup() {
	var err error

	if len(sc.Config.SecretsFile) > 0 {
		sc.Secrets, err = loadSecrets(sc.Config.SecretsFile)
		Expect(err).NotTo(HaveOccurred())
	} else {
		sc.Secrets = &CSISecrets{}
	}

	By("connecting to CSI driver")
	sc.Conn, err = utils.Connect(sc.Config.Address)
	Expect(err).NotTo(HaveOccurred())

	By("creating mount and staging directories")
	err = createMountTargetLocation(sc.Config.TargetPath)
	Expect(err).NotTo(HaveOccurred())
	if len(sc.Config.StagingPath) > 0 {
		err = createMountTargetLocation(sc.Config.StagingPath)
		Expect(err).NotTo(HaveOccurred())
	}
}

func (sc *SanityContext) teardown() {
	if sc.Conn != nil {
		sc.Conn.Close()
		sc.Conn = nil
	}
}

func createMountTargetLocation(targetPath string) error {
	fileInfo, err := os.Stat(targetPath)
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(targetPath, 0755)
	} else if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("Target location %s is not a directory", targetPath)
	}

	return nil
}

func loadSecrets(path string) (*CSISecrets, error) {
	var creds CSISecrets

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return &creds, fmt.Errorf("failed to read file %q: #%v", path, err)
	}

	err = yaml.Unmarshal(yamlFile, &creds)
	if err != nil {
		return &creds, fmt.Errorf("error unmarshaling yaml: #%v", err)
	}

	return &creds, nil
}

var uniqueSuffix = "-" + pseudoUUID()

// pseudoUUID returns a unique string generated from random
// bytes, empty string in case of error.
func pseudoUUID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Shouldn't happen?!
		return ""
	}
	return fmt.Sprintf("%08X-%08X", b[0:4], b[4:8])
}

// uniqueString returns a unique string by appending a random
// number. In case of an error, just the prefix is returned, so it
// alone should already be fairly unique.
func uniqueString(prefix string) string {
	return prefix + uniqueSuffix
}
