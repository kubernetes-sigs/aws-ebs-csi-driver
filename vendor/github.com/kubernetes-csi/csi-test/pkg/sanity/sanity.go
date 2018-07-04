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
	"fmt"
	"io/ioutil"
	"os"
	"sync"
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
}

var (
	config  *Config
	conn    *grpc.ClientConn
	lock    sync.Mutex
	secrets *CSISecrets
)

// Config provides the configuration for the sanity tests
type Config struct {
	TargetPath     string
	StagingPath    string
	Address        string
	SecretsFile    string
	TestVolumeSize int64
}

// Test will test the CSI driver at the specified address
func Test(t *testing.T, reqConfig *Config) {
	lock.Lock()
	defer lock.Unlock()

	config = reqConfig
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSI Driver Test Suite")
}

var _ = BeforeSuite(func() {
	var err error

	if len(config.SecretsFile) > 0 {
		secrets, err = loadSecrets(config.SecretsFile)
		Expect(err).NotTo(HaveOccurred())
	}

	By("connecting to CSI driver")
	conn, err = utils.Connect(config.Address)
	Expect(err).NotTo(HaveOccurred())

	By("creating mount and staging directories")
	err = createMountTargetLocation(config.TargetPath)
	Expect(err).NotTo(HaveOccurred())
	if len(config.StagingPath) > 0 {
		err = createMountTargetLocation(config.StagingPath)
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	conn.Close()
})

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
