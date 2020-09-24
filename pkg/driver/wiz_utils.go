package driver

import (
    "fmt"
    "github.com/diskfs/go-diskfs"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "k8s.io/klog"
    "k8s.io/utils/exec"
    "os"
    "path/filepath"
    "strings"
)

func appendMountOptions(fsType string, opts []string) []string {
    if fsType == FSTypeXfs {
        opts = append(opts, "nouuid")
    }

    return opts
}

func getFileSystemType(devicePath string, execer exec.Interface) (string, error) {
    args := []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", devicePath}
    klog.V(4).Infof("Attempting to determine if disk %q is formatted using blkid with args: (%v)", devicePath, args)
    dataOut, err := execer.Command("blkid", args...).CombinedOutput()
    output := string(dataOut)
    klog.V(4).Infof("Output: %q, err: %v", output, err)

    if err != nil {
        if exit, ok := err.(exec.ExitError); ok {
            if exit.ExitStatus() == 2 {
                // Disk device is unformatted.
                // For `blkid`, if the specified token (TYPE/PTTYPE, etc) was
                // not found, or no (specified) devices could be identified, an
                // exit code of 2 is returned.
                return "", nil
            }
        }
        klog.Errorf("Could not determine if disk %q is formatted (%v)", devicePath, err)
        return "", err
    }

    var fstype, pttype string

    lines := strings.Split(output, "\n")
    for _, l := range lines {
        if len(l) <= 0 {
            // Ignore empty line.
            continue
        }
        cs := strings.Split(l, "=")
        if len(cs) != 2 {
            return "", fmt.Errorf("blkid returns invalid output: %s", output)
        }
        // TYPE is filesystem type, and PTTYPE is partition table type, according
        // to https://www.kernel.org/pub/linux/utils/util-linux/v2.21/libblkid-docs/.
        if cs[0] == "TYPE" {
            fstype = cs[1]
        } else if cs[0] == "PTTYPE" {
            pttype = cs[1]
        }
    }

    if len(pttype) > 0 {
        klog.V(4).Infof("Disk %s detected partition table type: %s", devicePath, pttype)
        // Returns a special non-empty string as filesystem type, then kubelet
        // will not format it.
        return "unknown data, probably partitions", nil
    }

    return fstype, nil
}

func getPartitionedPath(devicePath string) (string, error) {
    absDevicePath, err := filepath.EvalSymlinks(devicePath)
    if err != nil {
        return absDevicePath, status.Errorf(codes.Internal, "Failed to follow symlink for path %s: %w", absDevicePath, err)
    }

    partitioned, err := isDiskPartitioned(absDevicePath)
    if err != nil {
        return absDevicePath, status.Errorf(codes.Internal, "Failed to check if disk is partitioned in path %s: %w", absDevicePath, err)
    }

    if partitioned {
        // Add the partition id to the end of the original disk location
        // On AWS the parition looks like: /dev/nvme1n1p1, so we'll check for both /dev/{device}{partitionid} and /dev/{device}p{partitionid}
        partitionId, err := getPartitionId(absDevicePath)
        probableAbsDiskLocation := fmt.Sprintf("%v%v", absDevicePath, partitionId)
        klog.V(4).Infof("Trying %v", probableAbsDiskLocation)

        if _, err = os.Stat(probableAbsDiskLocation); err != nil && os.IsNotExist(err) {
            probableAbsDiskLocation = fmt.Sprintf("%vp%v", absDevicePath, partitionId)
            klog.V(4).Infof("Trying %v", probableAbsDiskLocation)
            if _, err = os.Stat(probableAbsDiskLocation); err != nil && os.IsNotExist(err) {
                return absDevicePath, status.Error(codes.Internal, "couldn't find partition path")
            }
        }

        absDevicePath = probableAbsDiskLocation
    }

    return absDevicePath, nil
}

func isDiskPartitioned(diskLocation string) (bool, error) {
    files, err := filepath.Glob(diskLocation + "*")

    if err != nil {
        return false, fmt.Errorf("error in filepath.Glob(): %w", err)
    }

    return len(files) > 1, nil
}

func getPartitionId(diskPath string) (int, error) {
    disk, err := diskfs.OpenWithMode(diskPath, diskfs.ReadOnly)

    if err != nil {
        return 0, fmt.Errorf("error opening disk: %w", err)
    }

    //noinspection GoUnhandledErrorResult
    defer disk.File.Close()

    partitionTable, err := disk.GetPartitionTable()

    if err != nil {
        return 0, fmt.Errorf("error in GetPartitionTable(): %w", err)
    }

    klog.V(4).Infof("Partition table type: %v", partitionTable.Type())

    largestPartitionId := 0
    var largestPartitionSize int64 = 0

    for i, partition := range partitionTable.GetPartitions() {
        if partition.GetSize() > largestPartitionSize {
            largestPartitionSize = partition.GetSize()
            largestPartitionId = i
        }
    }

    // Effective partition (on disk) id is +1 from the one received here
    largestPartitionId += 1

    klog.V(4).Infof("Found largest partition: %v", largestPartitionId)

    return largestPartitionId, nil
}
