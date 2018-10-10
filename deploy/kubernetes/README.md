### Check Kubernetes Version
Since kubernetes v1.12, CSI driver uses [Kubelet Plugin Watcher](https://docs.google.com/document/d/1dtHpGY-gPe9sY7zzMGnm8Ywo09zJfNH-E1KEALFV39s/edit#heading=h.7fe6spexljh6) for driver registration. For kubernetes v1.10 - v1.11, use the example manifest files under v1.[10,11]. For kubernetes v1.12+, use example manifest files under v1.12+.

### Deploy the Driver
In order to use AWS EBS CSI driver, several sidecar containers and a secret is needed as follows:

1. Edit secret.yaml and put in your aws access key as key_id and aws secret key as access_key. Then:
   ```
   kubectl create -f secret.yaml
   ```
1. Deploy external provisioner, external attacher and driver registrar along with CSI driver:
   ```
   kubectl create -f provisioner.yaml
   kubectl create -f attacher.yaml
   kubectl create -f node.yaml
   ```

### Deploy Sample Application
1. Create storage class:
   ```
   kubectl create -f sample_app/storageclass.yaml
   ```
1. Create persistence volume claim:
   ```
   kubectl create -f sample_app/claim.yaml
   ```
1. Deploy application
   ```
   kubectl create -f sample_app/pod.yaml
   ```
