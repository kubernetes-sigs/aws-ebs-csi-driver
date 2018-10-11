### Deploy the driver
In order to use AWS EBS CSI driver, several sidecar containers and a secret is needed as follows:

1. Edit [secret.yaml](./secret.yaml) and put in your aws access key as key_id and aws secret key as access_key.
1. Deploy external provisioner, external attacher and driver registrar along with CSI driver:
   ```
   kuberctl create -f provisioner.yaml
   kuberctl create -f attacher.yaml
   kuberctl create -f node.yaml
   ```

### Deploy Sample Application
1. Create storage class:
   ```
   kuberctl create -f sample_app/storageclass.yaml
   ```
1. Create persistence volume claim:
   ```
   kuberctl create -f sample_app/claim.yaml
   ```
1. Deploy application
   ```
   kuberctl create -f sample_app/pod.yaml
   ```
