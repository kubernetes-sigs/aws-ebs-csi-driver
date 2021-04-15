/*
Copyright 2017 The Kubernetes Authors.

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
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	clientexec "k8s.io/client-go/util/exec"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2essh "k8s.io/kubernetes/test/e2e/framework/ssh"
	imageutils "k8s.io/kubernetes/test/utils/image"
	uexec "k8s.io/utils/exec"
)

// KubeletOpt type definition
type KubeletOpt string

const (
	// NodeStateTimeout defines Timeout
	NodeStateTimeout = 1 * time.Minute
	// KStart defines start value
	KStart KubeletOpt = "start"
	// KStop defines stop value
	KStop KubeletOpt = "stop"
	// KRestart defines restart value
	KRestart KubeletOpt = "restart"
)

const (
	// ClusterRole name for e2e test Priveledged Pod Security Policy User
	podSecurityPolicyPrivilegedClusterRoleName = "e2e-test-privileged-psp"
)

// PodExec runs f.ExecCommandInContainerWithFullOutput to execute a shell cmd in target pod
func PodExec(f *framework.Framework, pod *v1.Pod, shExec string) (string, string, error) {
	if framework.NodeOSDistroIs("windows") {
		return f.ExecCommandInContainerWithFullOutput(pod.Name, pod.Spec.Containers[0].Name, "powershell", "/c", shExec)
	}
	return f.ExecCommandInContainerWithFullOutput(pod.Name, pod.Spec.Containers[0].Name, "/bin/sh", "-c", shExec)

}

// VerifyExecInPodSucceed verifies shell cmd in target pod succeed
func VerifyExecInPodSucceed(f *framework.Framework, pod *v1.Pod, shExec string) {
	stdout, stderr, err := PodExec(f, pod, shExec)
	if err != nil {

		if exiterr, ok := err.(uexec.CodeExitError); ok {
			exitCode := exiterr.ExitStatus()
			framework.ExpectNoError(err,
				"%q should succeed, but failed with exit code %d and error message %q\nstdout: %s\nstderr: %s",
				shExec, exitCode, exiterr, stdout, stderr)
		} else {
			framework.ExpectNoError(err,
				"%q should succeed, but failed with error message %q\nstdout: %s\nstderr: %s",
				shExec, err, stdout, stderr)
		}
	}
}

// VerifyFSGroupInPod verifies that the passed in filePath contains the expectedFSGroup
func VerifyFSGroupInPod(f *framework.Framework, filePath, expectedFSGroup string, pod *v1.Pod) {
	cmd := fmt.Sprintf("ls -l %s", filePath)
	stdout, stderr, err := PodExec(f, pod, cmd)
	framework.ExpectNoError(err)
	framework.Logf("pod %s/%s exec for cmd %s, stdout: %s, stderr: %s", pod.Namespace, pod.Name, cmd, stdout, stderr)
	fsGroupResult := strings.Fields(stdout)[3]
	framework.ExpectEqual(expectedFSGroup, fsGroupResult,
		"Expected fsGroup of %s, got %s", expectedFSGroup, fsGroupResult)
}

// VerifyExecInPodFail verifies shell cmd in target pod fail with certain exit code
func VerifyExecInPodFail(f *framework.Framework, pod *v1.Pod, shExec string, exitCode int) {
	stdout, stderr, err := PodExec(f, pod, shExec)
	if err != nil {
		if exiterr, ok := err.(clientexec.ExitError); ok {
			actualExitCode := exiterr.ExitStatus()
			framework.ExpectEqual(actualExitCode, exitCode,
				"%q should fail with exit code %d, but failed with exit code %d and error message %q\nstdout: %s\nstderr: %s",
				shExec, exitCode, actualExitCode, exiterr, stdout, stderr)
		} else {
			framework.ExpectNoError(err,
				"%q should fail with exit code %d, but failed with error message %q\nstdout: %s\nstderr: %s",
				shExec, exitCode, err, stdout, stderr)
		}
	}
	framework.ExpectError(err, "%q should fail with exit code %d, but exit without error", shExec, exitCode)
}

func isSudoPresent(nodeIP string, provider string) bool {
	framework.Logf("Checking if sudo command is present")
	sshResult, err := e2essh.SSH("sudo --version", nodeIP, provider)
	framework.ExpectNoError(err, "SSH to %q errored.", nodeIP)
	if !strings.Contains(sshResult.Stderr, "command not found") {
		return true
	}
	return false
}

// getHostAddress gets the node for a pod and returns the first
// address. Returns an error if the node the pod is on doesn't have an
// address.
func getHostAddress(client clientset.Interface, p *v1.Pod) (string, error) {
	node, err := client.CoreV1().Nodes().Get(context.TODO(), p.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	// Try externalAddress first
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeExternalIP {
			if address.Address != "" {
				return address.Address, nil
			}
		}
	}
	// If no externalAddress found, try internalAddress
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			if address.Address != "" {
				return address.Address, nil
			}
		}
	}

	// If not found, return error
	return "", fmt.Errorf("No address for pod %v on node %v",
		p.Name, p.Spec.NodeName)
}

// KubeletCommand performs `start`, `restart`, or `stop` on the kubelet running on the node of the target pod and waits
// for the desired statues..
// - First issues the command via `systemctl`
// - If `systemctl` returns stderr "command not found, issues the command via `service`
// - If `service` also returns stderr "command not found", the test is aborted.
// Allowed kubeletOps are `KStart`, `KStop`, and `KRestart`
func KubeletCommand(kOp KubeletOpt, c clientset.Interface, pod *v1.Pod) {
	command := ""
	systemctlPresent := false
	kubeletPid := ""

	nodeIP, err := getHostAddress(c, pod)
	framework.ExpectNoError(err)
	nodeIP = nodeIP + ":22"

	framework.Logf("Checking if systemctl command is present")
	sshResult, err := e2essh.SSH("systemctl --version", nodeIP, framework.TestContext.Provider)
	framework.ExpectNoError(err, fmt.Sprintf("SSH to Node %q errored.", pod.Spec.NodeName))
	if !strings.Contains(sshResult.Stderr, "command not found") {
		command = fmt.Sprintf("systemctl %s kubelet", string(kOp))
		systemctlPresent = true
	} else {
		command = fmt.Sprintf("service kubelet %s", string(kOp))
	}

	sudoPresent := isSudoPresent(nodeIP, framework.TestContext.Provider)
	if sudoPresent {
		command = fmt.Sprintf("sudo %s", command)
	}

	if kOp == KRestart {
		kubeletPid = getKubeletMainPid(nodeIP, sudoPresent, systemctlPresent)
	}

	framework.Logf("Attempting `%s`", command)
	sshResult, err = e2essh.SSH(command, nodeIP, framework.TestContext.Provider)
	framework.ExpectNoError(err, fmt.Sprintf("SSH to Node %q errored.", pod.Spec.NodeName))
	e2essh.LogResult(sshResult)
	gomega.Expect(sshResult.Code).To(gomega.BeZero(), "Failed to [%s] kubelet:\n%#v", string(kOp), sshResult)

	if kOp == KStop {
		if ok := e2enode.WaitForNodeToBeNotReady(c, pod.Spec.NodeName, NodeStateTimeout); !ok {
			framework.Failf("Node %s failed to enter NotReady state", pod.Spec.NodeName)
		}
	}
	if kOp == KRestart {
		// Wait for a minute to check if kubelet Pid is getting changed
		isPidChanged := false
		for start := time.Now(); time.Since(start) < 1*time.Minute; time.Sleep(2 * time.Second) {
			kubeletPidAfterRestart := getKubeletMainPid(nodeIP, sudoPresent, systemctlPresent)
			if kubeletPid != kubeletPidAfterRestart {
				isPidChanged = true
				break
			}
		}
		framework.ExpectEqual(isPidChanged, true, "Kubelet PID remained unchanged after restarting Kubelet")
		framework.Logf("Noticed that kubelet PID is changed. Waiting for 30 Seconds for Kubelet to come back")
		time.Sleep(30 * time.Second)
	}
	if kOp == KStart || kOp == KRestart {
		// For kubelet start and restart operations, Wait until Node becomes Ready
		if ok := e2enode.WaitForNodeToBeReady(c, pod.Spec.NodeName, NodeStateTimeout); !ok {
			framework.Failf("Node %s failed to enter Ready state", pod.Spec.NodeName)
		}
	}
}

// getKubeletMainPid return the Main PID of the Kubelet Process
func getKubeletMainPid(nodeIP string, sudoPresent bool, systemctlPresent bool) string {
	command := ""
	if systemctlPresent {
		command = "systemctl status kubelet | grep 'Main PID'"
	} else {
		command = "service kubelet status | grep 'Main PID'"
	}
	if sudoPresent {
		command = fmt.Sprintf("sudo %s", command)
	}
	framework.Logf("Attempting `%s`", command)
	sshResult, err := e2essh.SSH(command, nodeIP, framework.TestContext.Provider)
	framework.ExpectNoError(err, fmt.Sprintf("SSH to Node %q errored.", nodeIP))
	e2essh.LogResult(sshResult)
	gomega.Expect(sshResult.Code).To(gomega.BeZero(), "Failed to get kubelet PID")
	gomega.Expect(sshResult.Stdout).NotTo(gomega.BeEmpty(), "Kubelet Main PID should not be Empty")
	return sshResult.Stdout
}

// TestKubeletRestartsAndRestoresMount tests that a volume mounted to a pod remains mounted after a kubelet restarts
func TestKubeletRestartsAndRestoresMount(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	path := "/mnt/volume1"
	byteLen := 64
	seed := time.Now().UTC().UnixNano()

	ginkgo.By("Writing to the volume.")
	CheckWriteToPath(f, clientPod, v1.PersistentVolumeFilesystem, false, path, byteLen, seed)

	ginkgo.By("Restarting kubelet")
	KubeletCommand(KRestart, c, clientPod)

	ginkgo.By("Testing that written file is accessible.")
	CheckReadFromPath(f, clientPod, v1.PersistentVolumeFilesystem, false, path, byteLen, seed)

	framework.Logf("Volume mount detected on pod %s and written file %s is readable post-restart.", clientPod.Name, path)
}

// TestKubeletRestartsAndRestoresMap tests that a volume mapped to a pod remains mapped after a kubelet restarts
func TestKubeletRestartsAndRestoresMap(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	path := "/mnt/volume1"
	byteLen := 64
	seed := time.Now().UTC().UnixNano()

	ginkgo.By("Writing to the volume.")
	CheckWriteToPath(f, clientPod, v1.PersistentVolumeBlock, false, path, byteLen, seed)

	ginkgo.By("Restarting kubelet")
	KubeletCommand(KRestart, c, clientPod)

	ginkgo.By("Testing that written pv is accessible.")
	CheckReadFromPath(f, clientPod, v1.PersistentVolumeBlock, false, path, byteLen, seed)

	framework.Logf("Volume map detected on pod %s and written data %s is readable post-restart.", clientPod.Name, path)
}

// TestVolumeUnmountsFromDeletedPodWithForceOption tests that a volume unmounts if the client pod was deleted while the kubelet was down.
// forceDelete is true indicating whether the pod is forcefully deleted.
// checkSubpath is true indicating whether the subpath should be checked.
func TestVolumeUnmountsFromDeletedPodWithForceOption(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod, forceDelete bool, checkSubpath bool) {
	nodeIP, err := getHostAddress(c, clientPod)
	framework.ExpectNoError(err)
	nodeIP = nodeIP + ":22"

	ginkgo.By("Expecting the volume mount to be found.")
	result, err := e2essh.SSH(fmt.Sprintf("mount | grep %s | grep -v volume-subpaths", clientPod.UID), nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	framework.ExpectEqual(result.Code, 0, fmt.Sprintf("Expected grep exit code of 0, got %d", result.Code))

	if checkSubpath {
		ginkgo.By("Expecting the volume subpath mount to be found.")
		result, err := e2essh.SSH(fmt.Sprintf("cat /proc/self/mountinfo | grep %s | grep volume-subpaths", clientPod.UID), nodeIP, framework.TestContext.Provider)
		e2essh.LogResult(result)
		framework.ExpectNoError(err, "Encountered SSH error.")
		framework.ExpectEqual(result.Code, 0, fmt.Sprintf("Expected grep exit code of 0, got %d", result.Code))
	}

	// This command is to make sure kubelet is started after test finishes no matter it fails or not.
	defer func() {
		KubeletCommand(KStart, c, clientPod)
	}()
	ginkgo.By("Stopping the kubelet.")
	KubeletCommand(KStop, c, clientPod)

	ginkgo.By(fmt.Sprintf("Deleting Pod %q", clientPod.Name))
	if forceDelete {
		err = c.CoreV1().Pods(clientPod.Namespace).Delete(context.TODO(), clientPod.Name, *metav1.NewDeleteOptions(0))
	} else {
		err = c.CoreV1().Pods(clientPod.Namespace).Delete(context.TODO(), clientPod.Name, metav1.DeleteOptions{})
	}
	framework.ExpectNoError(err)

	ginkgo.By("Starting the kubelet and waiting for pod to delete.")
	KubeletCommand(KStart, c, clientPod)
	err = e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, clientPod.Name, f.Namespace.Name, framework.PodDeleteTimeout)
	if err != nil {
		framework.ExpectNoError(err, "Expected pod to be not found.")
	}

	if forceDelete {
		// With forceDelete, since pods are immediately deleted from API server, there is no way to be sure when volumes are torn down
		// so wait some time to finish
		time.Sleep(30 * time.Second)
	}

	ginkgo.By("Expecting the volume mount not to be found.")
	result, err = e2essh.SSH(fmt.Sprintf("mount | grep %s | grep -v volume-subpaths", clientPod.UID), nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	gomega.Expect(result.Stdout).To(gomega.BeEmpty(), "Expected grep stdout to be empty (i.e. no mount found).")
	framework.Logf("Volume unmounted on node %s", clientPod.Spec.NodeName)

	if checkSubpath {
		ginkgo.By("Expecting the volume subpath mount not to be found.")
		result, err = e2essh.SSH(fmt.Sprintf("cat /proc/self/mountinfo | grep %s | grep volume-subpaths", clientPod.UID), nodeIP, framework.TestContext.Provider)
		e2essh.LogResult(result)
		framework.ExpectNoError(err, "Encountered SSH error.")
		gomega.Expect(result.Stdout).To(gomega.BeEmpty(), "Expected grep stdout to be empty (i.e. no subpath mount found).")
		framework.Logf("Subpath volume unmounted on node %s", clientPod.Spec.NodeName)
	}
}

// TestVolumeUnmountsFromDeletedPod tests that a volume unmounts if the client pod was deleted while the kubelet was down.
func TestVolumeUnmountsFromDeletedPod(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	TestVolumeUnmountsFromDeletedPodWithForceOption(c, f, clientPod, false, false)
}

// TestVolumeUnmountsFromForceDeletedPod tests that a volume unmounts if the client pod was forcefully deleted while the kubelet was down.
func TestVolumeUnmountsFromForceDeletedPod(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	TestVolumeUnmountsFromDeletedPodWithForceOption(c, f, clientPod, true, false)
}

// TestVolumeUnmapsFromDeletedPodWithForceOption tests that a volume unmaps if the client pod was deleted while the kubelet was down.
// forceDelete is true indicating whether the pod is forcefully deleted.
func TestVolumeUnmapsFromDeletedPodWithForceOption(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod, forceDelete bool) {
	nodeIP, err := getHostAddress(c, clientPod)
	framework.ExpectNoError(err, "Failed to get nodeIP.")
	nodeIP = nodeIP + ":22"

	// Creating command to check whether path exists
	podDirectoryCmd := fmt.Sprintf("ls /var/lib/kubelet/pods/%s/volumeDevices/*/ | grep '.'", clientPod.UID)
	if isSudoPresent(nodeIP, framework.TestContext.Provider) {
		podDirectoryCmd = fmt.Sprintf("sudo sh -c \"%s\"", podDirectoryCmd)
	}
	// Directories in the global directory have unpredictable names, however, device symlinks
	// have the same name as pod.UID. So just find anything with pod.UID name.
	globalBlockDirectoryCmd := fmt.Sprintf("find /var/lib/kubelet/plugins -name %s", clientPod.UID)
	if isSudoPresent(nodeIP, framework.TestContext.Provider) {
		globalBlockDirectoryCmd = fmt.Sprintf("sudo sh -c \"%s\"", globalBlockDirectoryCmd)
	}

	ginkgo.By("Expecting the symlinks from PodDeviceMapPath to be found.")
	result, err := e2essh.SSH(podDirectoryCmd, nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	framework.ExpectEqual(result.Code, 0, fmt.Sprintf("Expected grep exit code of 0, got %d", result.Code))

	ginkgo.By("Expecting the symlinks from global map path to be found.")
	result, err = e2essh.SSH(globalBlockDirectoryCmd, nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	framework.ExpectEqual(result.Code, 0, fmt.Sprintf("Expected find exit code of 0, got %d", result.Code))

	// This command is to make sure kubelet is started after test finishes no matter it fails or not.
	defer func() {
		KubeletCommand(KStart, c, clientPod)
	}()
	ginkgo.By("Stopping the kubelet.")
	KubeletCommand(KStop, c, clientPod)

	ginkgo.By(fmt.Sprintf("Deleting Pod %q", clientPod.Name))
	if forceDelete {
		err = c.CoreV1().Pods(clientPod.Namespace).Delete(context.TODO(), clientPod.Name, *metav1.NewDeleteOptions(0))
	} else {
		err = c.CoreV1().Pods(clientPod.Namespace).Delete(context.TODO(), clientPod.Name, metav1.DeleteOptions{})
	}
	framework.ExpectNoError(err, "Failed to delete pod.")

	ginkgo.By("Starting the kubelet and waiting for pod to delete.")
	KubeletCommand(KStart, c, clientPod)
	err = e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, clientPod.Name, f.Namespace.Name, framework.PodDeleteTimeout)
	framework.ExpectNoError(err, "Expected pod to be not found.")

	if forceDelete {
		// With forceDelete, since pods are immediately deleted from API server, there is no way to be sure when volumes are torn down
		// so wait some time to finish
		time.Sleep(30 * time.Second)
	}

	ginkgo.By("Expecting the symlink from PodDeviceMapPath not to be found.")
	result, err = e2essh.SSH(podDirectoryCmd, nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	gomega.Expect(result.Stdout).To(gomega.BeEmpty(), "Expected grep stdout to be empty.")

	ginkgo.By("Expecting the symlinks from global map path not to be found.")
	result, err = e2essh.SSH(globalBlockDirectoryCmd, nodeIP, framework.TestContext.Provider)
	e2essh.LogResult(result)
	framework.ExpectNoError(err, "Encountered SSH error.")
	gomega.Expect(result.Stdout).To(gomega.BeEmpty(), "Expected find stdout to be empty.")

	framework.Logf("Volume unmaped on node %s", clientPod.Spec.NodeName)
}

// TestVolumeUnmapsFromDeletedPod tests that a volume unmaps if the client pod was deleted while the kubelet was down.
func TestVolumeUnmapsFromDeletedPod(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	TestVolumeUnmapsFromDeletedPodWithForceOption(c, f, clientPod, false)
}

// TestVolumeUnmapsFromForceDeletedPod tests that a volume unmaps if the client pod was forcefully deleted while the kubelet was down.
func TestVolumeUnmapsFromForceDeletedPod(c clientset.Interface, f *framework.Framework, clientPod *v1.Pod) {
	TestVolumeUnmapsFromDeletedPodWithForceOption(c, f, clientPod, true)
}

// RunInPodWithVolume runs a command in a pod with given claim mounted to /mnt directory.
func RunInPodWithVolume(c clientset.Interface, ns, claimName, command string) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-volume-tester-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "volume-tester",
					Image:   imageutils.GetE2EImage(imageutils.BusyBox),
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", command},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "my-volume",
							MountPath: "/mnt/test",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "my-volume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
	pod, err := c.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	framework.ExpectNoError(err, "Failed to create pod: %v", err)
	defer func() {
		e2epod.DeletePodOrFail(c, ns, pod.Name)
	}()
	framework.ExpectNoError(e2epod.WaitForPodSuccessInNamespaceSlow(c, pod.Name, pod.Namespace))
}

// StartExternalProvisioner create external provisioner pod
func StartExternalProvisioner(c clientset.Interface, ns string, externalPluginName string) *v1.Pod {
	podClient := c.CoreV1().Pods(ns)

	provisionerPod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "external-provisioner-",
		},

		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nfs-provisioner",
					Image: imageutils.GetE2EImage(imageutils.NFSProvisioner),
					SecurityContext: &v1.SecurityContext{
						Capabilities: &v1.Capabilities{
							Add: []v1.Capability{"DAC_READ_SEARCH"},
						},
					},
					Args: []string{
						"-provisioner=" + externalPluginName,
						"-grace-period=0",
					},
					Ports: []v1.ContainerPort{
						{Name: "nfs", ContainerPort: 2049},
						{Name: "mountd", ContainerPort: 20048},
						{Name: "rpcbind", ContainerPort: 111},
						{Name: "rpcbind-udp", ContainerPort: 111, Protocol: v1.ProtocolUDP},
					},
					Env: []v1.EnvVar{
						{
							Name: "POD_IP",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "status.podIP",
								},
							},
						},
					},
					ImagePullPolicy: v1.PullIfNotPresent,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "export-volume",
							MountPath: "/export",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "export-volume",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	provisionerPod, err := podClient.Create(context.TODO(), provisionerPod, metav1.CreateOptions{})
	framework.ExpectNoError(err, "Failed to create %s pod: %v", provisionerPod.Name, err)

	framework.ExpectNoError(e2epod.WaitForPodRunningInNamespace(c, provisionerPod))

	ginkgo.By("locating the provisioner pod")
	pod, err := podClient.Get(context.TODO(), provisionerPod.Name, metav1.GetOptions{})
	framework.ExpectNoError(err, "Cannot locate the provisioner pod %v: %v", provisionerPod.Name, err)

	return pod
}

// PrivilegedTestPSPClusterRoleBinding test Pod Security Policy Role bindings
func PrivilegedTestPSPClusterRoleBinding(client clientset.Interface,
	namespace string,
	teardown bool,
	saNames []string) {
	bindingString := "Binding"
	if teardown {
		bindingString = "Unbinding"
	}
	roleBindingClient := client.RbacV1().RoleBindings(namespace)
	for _, saName := range saNames {
		ginkgo.By(fmt.Sprintf("%v priviledged Pod Security Policy to the service account %s", bindingString, saName))
		binding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "psp-" + saName,
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      saName,
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     podSecurityPolicyPrivilegedClusterRoleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		roleBindingClient.Delete(context.TODO(), binding.GetName(), metav1.DeleteOptions{})
		err := wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
			_, err := roleBindingClient.Get(context.TODO(), binding.GetName(), metav1.GetOptions{})
			return apierrors.IsNotFound(err), nil
		})
		framework.ExpectNoError(err, "Timed out waiting for RBAC binding %s deletion: %v", binding.GetName(), err)

		if teardown {
			continue
		}

		_, err = roleBindingClient.Create(context.TODO(), binding, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create %s role binding: %v", binding.GetName(), err)

	}
}

// CheckVolumeModeOfPath check mode of volume
func CheckVolumeModeOfPath(f *framework.Framework, pod *v1.Pod, volMode v1.PersistentVolumeMode, path string) {
	if volMode == v1.PersistentVolumeBlock {
		// Check if block exists
		VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -b %s", path))

		// Double check that it's not directory
		VerifyExecInPodFail(f, pod, fmt.Sprintf("test -d %s", path), 1)
	} else {
		// Check if directory exists
		VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -d %s", path))

		// Double check that it's not block
		VerifyExecInPodFail(f, pod, fmt.Sprintf("test -b %s", path), 1)
	}
}

// CheckReadWriteToPath check that path can b e read and written
func CheckReadWriteToPath(f *framework.Framework, pod *v1.Pod, volMode v1.PersistentVolumeMode, path string) {
	if volMode == v1.PersistentVolumeBlock {
		// random -> file1
		VerifyExecInPodSucceed(f, pod, "dd if=/dev/urandom of=/tmp/file1 bs=64 count=1")
		// file1 -> dev (write to dev)
		VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=/tmp/file1 of=%s bs=64 count=1", path))
		// dev -> file2 (read from dev)
		VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=%s of=/tmp/file2 bs=64 count=1", path))
		// file1 == file2 (check contents)
		VerifyExecInPodSucceed(f, pod, "diff /tmp/file1 /tmp/file2")
		// Clean up temp files
		VerifyExecInPodSucceed(f, pod, "rm -f /tmp/file1 /tmp/file2")

		// Check that writing file to block volume fails
		VerifyExecInPodFail(f, pod, fmt.Sprintf("echo 'Hello world.' > %s/file1.txt", path), 1)
	} else {
		// text -> file1 (write to file)
		VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo 'Hello world.' > %s/file1.txt", path))
		// grep file1 (read from file and check contents)
		VerifyExecInPodSucceed(f, pod, readFile("Hello word.", path))
		// Check that writing to directory as block volume fails
		VerifyExecInPodFail(f, pod, fmt.Sprintf("dd if=/dev/urandom of=%s bs=64 count=1", path), 1)
	}
}

func readFile(content, path string) string {
	if framework.NodeOSDistroIs("windows") {
		return fmt.Sprintf("Select-String '%s' %s/file1.txt", content, path)
	}
	return fmt.Sprintf("grep 'Hello world.' %s/file1.txt", path)
}

// genBinDataFromSeed generate binData with random seed
func genBinDataFromSeed(len int, seed int64) []byte {
	binData := make([]byte, len)
	rand.Seed(seed)

	_, err := rand.Read(binData)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	return binData
}

// CheckReadFromPath validate that file can be properly read.
//
// Note: directIO does not work with (default) BusyBox Pods. A requirement for
// directIO to function correctly, is to read whole sector(s) for Block-mode
// PVCs (normally a sector is 512 bytes), or memory pages for files (commonly
// 4096 bytes).
func CheckReadFromPath(f *framework.Framework, pod *v1.Pod, volMode v1.PersistentVolumeMode, directIO bool, path string, len int, seed int64) {
	var pathForVolMode string
	var iflag string

	if volMode == v1.PersistentVolumeBlock {
		pathForVolMode = path
	} else {
		pathForVolMode = filepath.Join(path, "file1.txt")
	}

	if directIO {
		iflag = "iflag=direct"
	}

	sum := sha256.Sum256(genBinDataFromSeed(len, seed))

	VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=%s %s bs=%d count=1 | sha256sum", pathForVolMode, iflag, len))
	VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=%s %s bs=%d count=1 | sha256sum | grep -Fq %x", pathForVolMode, iflag, len, sum))
}

// CheckWriteToPath that file can be properly written.
//
// Note: nocache does not work with (default) BusyBox Pods. To read without
// caching, enable directIO with CheckReadFromPath and check the hints about
// the len requirements.
func CheckWriteToPath(f *framework.Framework, pod *v1.Pod, volMode v1.PersistentVolumeMode, nocache bool, path string, len int, seed int64) {
	var pathForVolMode string
	var oflag string

	if volMode == v1.PersistentVolumeBlock {
		pathForVolMode = path
	} else {
		pathForVolMode = filepath.Join(path, "file1.txt")
	}

	if nocache {
		oflag = "oflag=nocache"
	}

	encoded := base64.StdEncoding.EncodeToString(genBinDataFromSeed(len, seed))

	VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo %s | base64 -d | sha256sum", encoded))
	VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo %s | base64 -d | dd of=%s %s bs=%d count=1", encoded, pathForVolMode, oflag, len))
}

// findMountPoints returns all mount points on given node under specified directory.
func findMountPoints(hostExec HostExec, node *v1.Node, dir string) []string {
	result, err := hostExec.IssueCommandWithResult(fmt.Sprintf(`find %s -type d -exec mountpoint {} \; | grep 'is a mountpoint$' || true`, dir), node)
	framework.ExpectNoError(err, "Encountered HostExec error.")
	var mountPoints []string
	if err != nil {
		for _, line := range strings.Split(result, "\n") {
			if line == "" {
				continue
			}
			mountPoints = append(mountPoints, strings.TrimSuffix(line, " is a mountpoint"))
		}
	}
	return mountPoints
}

// FindVolumeGlobalMountPoints returns all volume global mount points on the node of given pod.
func FindVolumeGlobalMountPoints(hostExec HostExec, node *v1.Node) sets.String {
	return sets.NewString(findMountPoints(hostExec, node, "/var/lib/kubelet/plugins")...)
}

// CreateDriverNamespace creates a namespace for CSI driver installation.
// The namespace is still tracked and ensured that gets deleted when test terminates.
func CreateDriverNamespace(f *framework.Framework) *v1.Namespace {
	ginkgo.By(fmt.Sprintf("Building a driver namespace object, basename %s", f.Namespace.Name))
	// The driver namespace will be bound to the test namespace in the prefix
	namespace, err := f.CreateNamespace(f.Namespace.Name, map[string]string{
		"e2e-framework":      f.BaseName,
		"e2e-test-namespace": f.Namespace.Name,
	})
	framework.ExpectNoError(err)

	if framework.TestContext.VerifyServiceAccount {
		ginkgo.By("Waiting for a default service account to be provisioned in namespace")
		err = framework.WaitForDefaultServiceAccountInNamespace(f.ClientSet, namespace.Name)
		framework.ExpectNoError(err)
	} else {
		framework.Logf("Skipping waiting for service account")
	}
	return namespace
}

// WaitForGVRDeletion waits until a non-namespaced object has been deleted
func WaitForGVRDeletion(c dynamic.Interface, gvr schema.GroupVersionResource, objectName string, poll, timeout time.Duration) error {
	framework.Logf("Waiting up to %v for %s %s to be deleted", timeout, gvr.Resource, objectName)

	if successful := WaitUntil(poll, timeout, func() bool {
		_, err := c.Resource(gvr).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			framework.Logf("%s %v is not found and has been deleted", gvr.Resource, objectName)
			return true
		} else if err != nil {
			framework.Logf("Get %s returned an error: %v", objectName, err.Error())
		} else {
			framework.Logf("%s %v has been found and is not deleted", gvr.Resource, objectName)
		}

		return false
	}); successful {
		return nil
	}

	return fmt.Errorf("%s %s is not deleted within %v", gvr.Resource, objectName, timeout)
}

// WaitForNamespacedGVRDeletion waits until a namespaced object has been deleted
func WaitForNamespacedGVRDeletion(c dynamic.Interface, gvr schema.GroupVersionResource, ns, objectName string, poll, timeout time.Duration) error {
	framework.Logf("Waiting up to %v for %s %s to be deleted", timeout, gvr.Resource, objectName)

	if successful := WaitUntil(poll, timeout, func() bool {
		_, err := c.Resource(gvr).Namespace(ns).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			framework.Logf("%s %s is not found in namespace %s and has been deleted", gvr.Resource, objectName, ns)
			return true
		} else if err != nil {
			framework.Logf("Get %s in namespace %s returned an error: %v", objectName, ns, err.Error())
		} else {
			framework.Logf("%s %s has been found in namespace %s and is not deleted", gvr.Resource, objectName, ns)
		}

		return false
	}); successful {
		return nil
	}

	return fmt.Errorf("%s %s in namespace %s is not deleted within %v", gvr.Resource, objectName, ns, timeout)
}

// WaitUntil runs checkDone until a timeout is reached
func WaitUntil(poll, timeout time.Duration, checkDone func() bool) bool {
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		if checkDone() {
			framework.Logf("WaitUntil finished successfully after %v", time.Since(start))
			return true
		}
	}

	framework.Logf("WaitUntil failed after reaching the timeout %v", timeout)
	return false
}

// WaitForGVRFinalizer waits until a object from a given GVR contains a finalizer
// If namespace is empty, assume it is a non-namespaced object
func WaitForGVRFinalizer(ctx context.Context, c dynamic.Interface, gvr schema.GroupVersionResource, objectName, objectNamespace, finalizer string, poll, timeout time.Duration) error {
	framework.Logf("Waiting up to %v for object %s %s of resource %s to contain finalizer %s", timeout, objectNamespace, objectName, gvr.Resource, finalizer)
	var (
		err      error
		resource *unstructured.Unstructured
	)
	if successful := WaitUntil(poll, timeout, func() bool {
		switch objectNamespace {
		case "":
			resource, err = c.Resource(gvr).Get(ctx, objectName, metav1.GetOptions{})
		default:
			resource, err = c.Resource(gvr).Namespace(objectNamespace).Get(ctx, objectName, metav1.GetOptions{})
		}
		if err != nil {
			framework.Logf("Failed to get object %s %s with err: %v. Will retry in %v", objectNamespace, objectName, err, timeout)
			return false
		}
		for _, f := range resource.GetFinalizers() {
			if f == finalizer {
				return true
			}
		}
		return false
	}); successful {
		return nil
	}
	if err == nil {
		err = fmt.Errorf("finalizer %s not added to object %s %s of resource %s", finalizer, objectNamespace, objectName, gvr)
	}
	return err
}

// VerifyFilePathGidInPod verfies expected GID of the target filepath
func VerifyFilePathGidInPod(f *framework.Framework, filePath, expectedGid string, pod *v1.Pod) {
	cmd := fmt.Sprintf("ls -l %s", filePath)
	stdout, stderr, err := PodExec(f, pod, cmd)
	framework.ExpectNoError(err)
	framework.Logf("pod %s/%s exec for cmd %s, stdout: %s, stderr: %s", pod.Namespace, pod.Name, cmd, stdout, stderr)
	ll := strings.Fields(stdout)
	framework.Logf("stdout split: %v, expected gid: %v", ll, expectedGid)
	framework.ExpectEqual(ll[3], expectedGid)
}

// ChangeFilePathGidInPod changes the GID of the target filepath.
func ChangeFilePathGidInPod(f *framework.Framework, filePath, targetGid string, pod *v1.Pod) {
	cmd := fmt.Sprintf("chgrp %s %s", targetGid, filePath)
	_, _, err := PodExec(f, pod, cmd)
	framework.ExpectNoError(err)
	VerifyFilePathGidInPod(f, filePath, targetGid, pod)
}
