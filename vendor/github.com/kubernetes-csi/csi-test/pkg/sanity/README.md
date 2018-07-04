# CSI Driver Sanity Tester

This library provides a simple way to ensure that a CSI driver conforms to
the CSI specification. There are two ways to leverage this testing framework.
For CSI drivers written in Golang, the framework provides a simple API function
to call to test the driver. Another way to run the test suite is to use the
command line program [csi-sanity](https://github.com/kubernetes-csi/csi-test/tree/master/cmd/csi-sanity).

## For Golang CSI Drivers
This framework leverages the Ginkgo BDD testing framework to deliver a descriptive
test suite for your driver. To test your driver, simply call the API in one of your
Golang `TestXXX` functions. For example:

```go
func TestMyDriver(t *testing.T) {
    // Setup the full driver and its environment
    ... setup driver ...

    // Now call the test suite
    sanity.Test(t, driverEndpointAddress, "/mnt")
}
```

## Command line program
Please see [csi-sanity](https://github.com/kubernetes-csi/csi-test/tree/master/cmd/csi-sanity)
