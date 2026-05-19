# EBS CSI Driver FIPS 140-3 Support

## Overview

The EBS CSI Driver is built with Go's native FIPS 140-3 cryptographic module ([CMVP Certificate #5247](https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/5247)). FIPS mode is disabled by default and can be activated at runtime without requiring a separate image.

## Enabling FIPS Mode

### Helm

Set `fips: true` in your Helm values. This activates two things:

1. **FIPS cryptography** via `GODEBUG=fips140=on`, which enables the Go Cryptographic Module's FIPS 140-3 mode (integrity checks, approved-only algorithms in `crypto/tls`, DRBG-backed `crypto/rand`).
2. **FIPS endpoints** via `AWS_USE_FIPS_ENDPOINT=true`, which instructs the AWS SDK to use FIPS-validated API endpoints.

FIPS endpoints are only available in some regions. See [AWS FIPS documentation](https://aws.amazon.com/compliance/fips/) for regional availability.

### Kustomize

Add the following environment variables to the controller and node DaemonSet containers:

```yaml
env:
  - name: GODEBUG
    value: "fips140=on"
  - name: AWS_USE_FIPS_ENDPOINT
    value: "true"
```

## How It Works

The driver binary is compiled with `GOFIPS140=certified`, which embeds Go's certified FIPS 140-3 cryptographic module. The Dockerfile sets `GODEBUG=fips140=off` by default, so FIPS mode remains inactive unless explicitly enabled via the pod environment.

When FIPS mode is active:
- The cryptographic module performs integrity self-checks and known-answer tests at startup
- `crypto/tls` restricts connections to FIPS-approved cipher suites and protocol versions
- `crypto/rand` uses a NIST SP 800-90A Rev 1 DRBG

## Disclaimer

The EBS CSI Driver itself has not undergone FIPS certification. No official guarantee is made about the compliance of this software under the FIPS standard. Users relying on this for FIPS compliance should perform their own independent evaluation.
