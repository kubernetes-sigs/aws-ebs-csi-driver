package e2e_upgrade

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubernetes/test/e2e/framework"
	frameworkconfig "k8s.io/kubernetes/test/e2e/framework/config"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

var (
	kopsBinaryPath  = flag.String("binary", "kops", "")
	kopsStateStore  = flag.String("state", "s3://k8s-kops-csi-e2e", "")
	kopsClusterName = flag.String("name", "", "")
)

// TestKopsMigration tests the configurations described here:
// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/csi-migration.md#upgradedowngradeskew-testing
func TestKopsMigration(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "TestKopsMigration")
}

func init() {
	rand.Seed(time.Now().UnixNano())

	testing.Init()
	frameworkconfig.CopyFlags(frameworkconfig.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)

	flag.Parse()

	if home := homedir.HomeDir(); home != "" && framework.TestContext.KubeConfig == "" {
		framework.TestContext.KubeConfig = filepath.Join(home, ".kube", "config")
	}
	framework.AfterReadingAllFlags(&framework.TestContext)

	ginkgo.Describe("Kops", func() {
		var (
			k         *kops
			clientset *kubernetes.Clientset
			f         *framework.Framework
		)
		k = &kops{*kopsBinaryPath, *kopsStateStore, *kopsClusterName}
		var err error
		clientset, err = k.exportKubecfg(framework.TestContext.KubeConfig)
		if err != nil {
			panic(err)
		}
		f = framework.NewFramework("kops-migrate", framework.Options{}, clientset)

		ginkgo.It("should call csi plugin for all operations after migration toggled on", func() {
			migrationOn := false
			toggleMigration(k, migrationOn)
			_, inTreePVC, _ := createAndVerify(k, migrationOn, f, nil)
			// Don't deleteAndVerify, the other test covers it. The inTreePod will
			// get evicted & deleted as part of the following toggle

			migrationOn = true
			toggleMigration(k, migrationOn)
			csiPod, csiPVC, csiPV := createAndVerify(k, migrationOn, f, inTreePVC)
			deleteAndVerify(f, migrationOn, csiPod, csiPVC, csiPV)
		})

		ginkgo.It("should call in-tree plugin for all operations after migration toggled off", func() {
			migrationOn := true
			toggleMigration(k, migrationOn)
			_, csiPVC, _ := createAndVerify(k, migrationOn, f, nil)
			// Don't deleteAndVerify, the other test covers it. The csiPod will
			// get evicted & deleted as part of the following toggle

			migrationOn = false
			toggleMigration(k, migrationOn)
			inTreePod, inTreePVC, inTreePV := createAndVerify(k, migrationOn, f, csiPVC)
			deleteAndVerify(f, migrationOn, inTreePod, inTreePVC, inTreePV)
		})

		/*
			ginkgo.It("should call in-tree plugin for attach & mount and csi plugin for provision after kube-controller-manager migration toggled on and kubelet migration toggled off", func() {
				// TODO
			})
		*/
	})
}

var (
	csiVerifier = verifier{
		name:        "csi",
		provisioner: "ebs.csi.aws.com",
		plugin:      "kubernetes.io/csi/ebs.csi.aws.com",
	}
	inTreeVerifier = verifier{
		name:        "in-tree",
		provisioner: "kubernetes.io/aws-ebs",
		plugin:      "kubernetes.io/aws-ebs",
	}
)

func toggleMigration(k *kops, migrationOn bool) {
	var step string
	var err error
	if migrationOn {
		step = "Toggling kube-controller-manager migration ON"
		ginkgo.By(step)
		err = k.toggleMigration("kubeControllerManager", migrationOn)
		framework.ExpectNoError(err, step)

		step = "Toggling kubelet migration ON"
		ginkgo.By(step)
		err = k.toggleMigration("kubelet", migrationOn)
		framework.ExpectNoError(err, step)
	} else {
		step = "Toggling kubelet migration OFF"
		ginkgo.By(step)
		err = k.toggleMigration("kubelet", migrationOn)
		framework.ExpectNoError(err, step)

		step = "Toggling kube-controller-manager migration OFF"
		ginkgo.By(step)
		err = k.toggleMigration("kubeControllerManager", migrationOn)
		framework.ExpectNoError(err, step)
	}
}

// createAndVerify creates a pod + pvc and verifies that csi/in-tree does
// operations according to whether migrationOn is true/false. optionally
// accepts a preTogglePVC to verify the same for a pvc that already existed
// prior to migration being toggled on/off
func createAndVerify(k *kops, migrationOn bool, f *framework.Framework, preTogglePVC *v1.PersistentVolumeClaim) (*v1.Pod, *v1.PersistentVolumeClaim, *v1.PersistentVolume) {
	var step string
	var err error
	var v *verifier
	if migrationOn {
		v = &csiVerifier
	} else {
		v = &inTreeVerifier
	}

	clientset, err := k.exportKubecfg(framework.TestContext.KubeConfig)
	f.ClientSet = clientset

	if preTogglePVC != nil {
		step = "Creating post-toggle Pod using pre-toggle PVC"
		ginkgo.By(step)
		extraPod, _, preTogglePV, err := createPodPVC(f, preTogglePVC)
		framework.ExpectNoError(err, step)

		step = fmt.Sprintf("Verifying pre-toggle PV %q got re-attached by %s", preTogglePV.Name, v.name)
		ginkgo.By(step)
		err = v.verifyAttach(f, preTogglePV)
		framework.ExpectNoError(err, step)

		step = fmt.Sprintf("Verifying pre-toggle PV %q got re-mounted by %s", preTogglePV.Name, v.name)
		ginkgo.By(step)
		err = v.verifyMount(f, preTogglePV, extraPod.Spec.NodeName)
		framework.ExpectNoError(err, step)

		step = fmt.Sprintf("Deleting pod %q", extraPod.Name)
		ginkgo.By(step)
		err = e2epod.DeletePodWithWait(f.ClientSet, extraPod)
		framework.ExpectNoError(err, step)
	}

	step = "Creating post-toggle Pod using post-toggle PVC"
	ginkgo.By(step)
	pod, pvc, pv, err := createPodPVC(f, nil)
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Verifying post-toggle PV %q got attached by %s", pv.Name, v.name)
	ginkgo.By(step)
	err = v.verifyAttach(f, pv)
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Verifying post-toggle PV %q got mounted by %s", pv.Name, v.name)
	ginkgo.By(step)
	err = v.verifyMount(f, pv, pod.Spec.NodeName)
	framework.ExpectNoError(err, step)

	return pod, pvc, pv
}

// deleteAndVerify deletes a pod + pvc and verifies that csi/in-tree does
// operations according to whether migrationOn is true/false
func deleteAndVerify(f *framework.Framework, migrationOn bool, pod *v1.Pod, pvc *v1.PersistentVolumeClaim, pv *v1.PersistentVolume) {
	var step string
	var err error
	var v *verifier
	if migrationOn {
		v = &csiVerifier
	} else {
		v = &inTreeVerifier
	}

	step = fmt.Sprintf("Deleting Pod %q", pod.Name)
	ginkgo.By(step)
	err = e2epod.DeletePodWithWait(f.ClientSet, pod)
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Deleting PVC %q", pvc.Name)
	ginkgo.By(step)
	err = f.ClientSet.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Waiting for PV %q to be deleted", pv.Name)
	ginkgo.By(step)
	err = f.ClientSet.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
	err = e2epv.WaitForPersistentVolumeDeleted(f.ClientSet, pvc.Spec.VolumeName, 30*time.Second, 2*time.Minute)
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Verifying PV %q got unmounted by %s", pv.Name, v.name)
	ginkgo.By(step)
	err = v.verifyUnmount(f, pv, pod.Spec.NodeName)
	framework.ExpectNoError(err, step)

	step = fmt.Sprintf("Verifying PV %q got detached by %s", pv.Name, v.name)
	ginkgo.By(step)
	err = v.verifyDetach(f, pv)
	framework.ExpectNoError(err, step)
}

func createPodPVC(f *framework.Framework, pvc *v1.PersistentVolumeClaim) (*v1.Pod, *v1.PersistentVolumeClaim, *v1.PersistentVolume, error) {
	clientset := f.ClientSet
	ns := f.Namespace.Name
	var err error

	if pvc == nil {
		pvc = &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: f.BaseName,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		pvc, err = clientset.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvc, metav1.CreateOptions{})
		if err != nil {
			return nil, nil, nil, err
		}
	}

	pod := e2epod.MakePod(ns, nil, []*v1.PersistentVolumeClaim{pvc}, false, "")
	pod, err = clientset.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	err = e2epod.WaitForPodNameRunningInNamespace(clientset, pod.Name, ns)
	if err != nil {
		return nil, nil, nil, err
	}

	pod, err = clientset.CoreV1().Pods(ns).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	pvc, err = clientset.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	pv, err := clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	return pod, pvc, pv, nil
}

type verifier struct {
	name        string
	provisioner string
	plugin      string
}

/*
func (v *verifier) verifyProvision(pv *v1.PersistentVolume) error {
	provisionedBy, ok := pv.Annotations["pv.kubernetes.io/provisioned-by"]
	if !ok {
		return errors.New("provisioned-by annotation missing")
	} else if provisionedBy != v.provisioner {
		return fmt.Errorf("provisioned-by annotation is %q but expected %q", provisionedBy, v.provisioner)
	}
	return nil
}

// TODO verifyProvision/verifyDelete: check csi pod logs or kcm logs, relying on provisioned-by will break soon https://github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/pull/104
*/

func (v *verifier) verifyAttach(f *framework.Framework, pv *v1.PersistentVolume) error {
	re := regexp.MustCompile(fmt.Sprintf("AttachVolume.Attach.*%s.*%s", v.plugin, volumeID(pv)))
	return findKubeControllerManagerLogs(f.ClientSet, re)
}

func (v *verifier) verifyDetach(f *framework.Framework, pv *v1.PersistentVolume) error {
	re := regexp.MustCompile(fmt.Sprintf("DetachVolume.Detach.*%s.*%s", v.plugin, volumeID(pv)))
	return findKubeControllerManagerLogs(f.ClientSet, re)
}

func (v *verifier) verifyMount(f *framework.Framework, pv *v1.PersistentVolume, nodeName string) error {
	re := regexp.MustCompile(fmt.Sprintf("MountVolume.Mount.*%s.*%s", v.plugin, volumeID(pv)))
	return findKubeletLogs(f, nodeName, re)
}

func (v *verifier) verifyUnmount(f *framework.Framework, pv *v1.PersistentVolume, nodeName string) error {
	re := regexp.MustCompile(fmt.Sprintf("UnmountVolume.TearDown.*%s.*%s", v.plugin, volumeID(pv)))
	return findKubeletLogs(f, nodeName, re)
}

func volumeID(pv *v1.PersistentVolume) string {
	segments := strings.Split(pv.Spec.AWSElasticBlockStore.VolumeID, "/")
	return segments[len(segments)-1]
}

func findKubeletLogs(f *framework.Framework, nodeName string, re *regexp.Regexp) error {
	logs, err := kubeletLogs(f, nodeName)
	if err != nil {
		return fmt.Errorf("error getting kubelet logs: %v", err)
	}
	match := re.FindString(logs)
	if match == "" {
		return fmt.Errorf("regexp %q not found", re)
	}
	return nil
}

func findKubeControllerManagerLogs(clientset kubernetes.Interface, re *regexp.Regexp) error {
	logs, err := kubeControllerManagerLogs(clientset)
	if err != nil {
		return fmt.Errorf("error getting kube-controller-manager logs: %v", err)
	}
	match := re.FindString(logs)
	if match == "" {
		return fmt.Errorf("regexp %q not found", re)
	}
	return nil
}

func kubeletLogs(f *framework.Framework, nodeName string) (string, error) {
	hostExec := utils.NewHostExec(f)
	node, err := f.ClientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	logs, err := hostExec.IssueCommandWithResult("journalctl -u kubelet", node)
	if err != nil {
		return "", err
	}
	return logs, nil
}

func kubeControllerManagerLogs(clientset kubernetes.Interface) (string, error) {
	return podLogs(clientset, "kube-controller-manager")
}

func podLogs(clientset kubernetes.Interface, podNamePrefix string) (string, error) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceSystem).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, podNamePrefix) {
			body, err := clientset.CoreV1().Pods(metav1.NamespaceSystem).GetLogs(pod.Name, &v1.PodLogOptions{}).Do(context.TODO()).Raw()
			if err != nil {
				return "", err
			}
			return string(body), nil
		}
	}
	return "", fmt.Errorf("%q pod not found", podNamePrefix)
}
