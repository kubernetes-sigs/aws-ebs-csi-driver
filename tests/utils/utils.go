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

package utils

import (
	"fmt"
	"io/ioutil"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	namespace = "default"
)

type Kubectl struct {
	Kubeconfig string
	clientset  *kubernetes.Clientset
}

func NewKubectl(kubeconfig string) (*Kubectl, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Kubectl{
		Kubeconfig: kubeconfig,
		clientset:  clientset,
	}, nil
}

func (kk *Kubectl) Create(filePath string) error {
	var err error
	spec, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	decode := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decode.Decode(spec, nil, nil)

	switch o := obj.(type) {
	case *corev1.Pod:
		_, err = kk.clientset.CoreV1().Pods(namespace).Create(o)
	case *corev1.PersistentVolumeClaim:
		_, err = kk.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(o)
	case *storagev1.StorageClass:
		_, err = kk.clientset.StorageV1().StorageClasses().Create(o)
	}

	return err
}

func (kk *Kubectl) Delete(filePath string) error {
	var err error
	spec, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	decode := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decode.Decode(spec, nil, nil)

	switch o := obj.(type) {
	case *corev1.Pod:
		err = kk.clientset.CoreV1().Pods(namespace).Delete(o.Name, &metav1.DeleteOptions{})
	case *corev1.PersistentVolumeClaim:
		err = kk.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(o.Name, &metav1.DeleteOptions{})
	case *storagev1.StorageClass:
		err = kk.clientset.StorageV1().StorageClasses().Delete(o.Name, &metav1.DeleteOptions{})
	}

	return err
}

func (kk *Kubectl) EnsurePodRunning(filePath string) error {
	var err error
	spec, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	decode := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decode.Decode(spec, nil, nil)
	if err != nil {
		return err
	}

	podObj, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("Non Pod Spec at: %v", filePath)
	}

	var (
		checkInterval = 1 * time.Second
		checkTimeout  = 60 * time.Second
	)

	return wait.Poll(checkInterval, checkTimeout, func() (done bool, err error) {
		pod, err := kk.clientset.CoreV1().Pods(namespace).Get(podObj.Name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		if pod.Status.Phase == corev1.PodRunning {
			return true, nil
		}

		return false, nil
	})
}
