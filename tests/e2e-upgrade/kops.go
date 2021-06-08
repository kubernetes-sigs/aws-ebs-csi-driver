package e2e_upgrade

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type kops struct {
	kopsBinaryPath  string
	kopsStateStore  string
	kopsClusterName string
}

func (k *kops) commandQuiet(args ...string) *exec.Cmd {
	args = append(args, "--state", k.kopsStateStore, "--name", k.kopsClusterName)
	cmd := exec.Command(k.kopsBinaryPath, args...)
	log.Print(cmd.String())
	return cmd
}

func (k *kops) command(args ...string) *exec.Cmd {
	cmd := k.commandQuiet(args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (k *kops) getCluster() (*unstructured.Unstructured, error) {
	cmd := k.commandQuiet(
		"get",
		"cluster",
		"-o",
		"json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("out: %v, err: %v", string(out), err)
	}

	// kops client/API takes a long time to compile so settle for kops cli and unstructured
	cluster := new(unstructured.Unstructured)
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(out, nil, cluster)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// similar to kubetest2 code
// https://github.com/kubernetes/kops/tree/master/tests/e2e/kubetest2-kops/deployer
func (k *kops) updateCluster(cluster *unstructured.Unstructured) error {
	tmpfile, err := ioutil.TempFile("", "cluster")
	if err != nil {
		log.Fatal(err)
	}
	err = unstructured.UnstructuredJSONScheme.Encode(cluster, tmpfile)
	if err != nil {
		log.Fatal(err)
	}

	cmd := k.command(
		"replace",
		"-f",
		tmpfile.Name(),
	)
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = k.command(
		"update",
		"cluster",
		"--yes",
	)
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = k.command(
		"rolling-update",
		"cluster",
		"--yes",
	)
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (k *kops) exportKubecfg(kubeconfig string) (*kubernetes.Clientset, error) {
	cmd := k.command(
		"export",
		"kubecfg",
		"--admin",
		"--kubeconfig",
		kubeconfig,
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func (k *kops) setFeatureGate(cluster *unstructured.Unstructured, component, featureGate string, on bool) (*unstructured.Unstructured, error) {
	featureGates, found, err := unstructured.NestedMap(cluster.Object, "spec", component, "featureGates")
	if err != nil {
		return nil, err
	}
	if !found {
		featureGates = make(map[string]interface{})
	}

	featureGates[featureGate] = strconv.FormatBool(on)

	err = unstructured.SetNestedMap(cluster.Object, featureGates, "spec", component, "featureGates")
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func (k *kops) toggleMigration(component string, on bool) error {
	cluster, err := k.getCluster()
	if err != nil {
		return err
	}

	cluster, err = k.setFeatureGate(cluster, component, "CSIMigration", on)
	if err != nil {
		return err
	}

	cluster, err = k.setFeatureGate(cluster, component, "CSIMigrationAWS", on)
	if err != nil {
		return err
	}

	err = k.updateCluster(cluster)
	if err != nil {
		return err
	}

	return nil
}
