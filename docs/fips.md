# EBS CSI Driver FIPS Support

## Support

The EBS CSI Driver Helm chart can be configured to enable two modifications to better support environments that require FIPS certification. Both of these modifications are activated by changing the Helm parameter `fips` from `false` to `true`.

### FIPS Endpoints

The AWS SDK will be instructed to use FIPS endpoints [via the `AWS_USE_FIPS_ENDPOINT` environment variable](https://docs.aws.amazon.com/sdkref/latest/guide/feature-endpoints.html). FIPS endpoints are only supported in some regions, and thus the option will only work in regions that have both an STS and EC2 FIPS endpoint available. For a full list of current regions with FIPS endpoints available, see [the FIPS section of the AWS documentation](https://aws.amazon.com/compliance/fips/).

### FIPS Image

The EBS CSI Driver image will be swapped with an image built using BoringCrypto as Go's cryptographic library. BoringCrypto has [an active FIPS 140-3 certification](https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4735).

The EBS CSI Driver FIPS images have not undergone FIPS certification, and no official guarantee is made about the compliance of these images under the FIPS standard. Users relying on these images for FIPS compliance should perform their own independent evaluation.
