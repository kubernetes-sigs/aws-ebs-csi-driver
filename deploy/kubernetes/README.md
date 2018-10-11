### Deploy the driver
In order to use AWS EBS CSI driver, several sidecar containers and a secret is needed as follows:

1. Edit [secret.yaml](./secret.yaml) and put in your aws access key as key_id and aws secret key as access_key.
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
