# AWS EBS CSI Driver for Kubernetes

## Overview

The AWS EBS CSI Driver is used by Container Orchestrators (COs) to manage the lifecycle of AWS Elastic Block Storage Volumes. It is compliant to the [Container Storage Interface (CSI) Specification](https://github.com/container-storage-interface/spec/blob/master/spec.md).

## EBS volume lifecycle management on Kubernetes

This section describes how the AWS EBS CSI Driver is deployed on Kubernetes. We recommend that you review the Kubernetes CSI developer documentation's [introduction](https://kubernetes-csi.github.io/docs/introduction.html) and [deployment overview](https://kubernetes-csi.github.io/docs/deploying.html) before reading further.

The AWS EBS CSI Driver's `ebs-plugin` container implements the CSI Remote Procedure Calls (RPCs) that enable COs like Kubernetes to manage EBS volume lifecycles. This container implements both the Controller Service and Node Service RPCs defined by the CSI specification, with its operational mode determined by startup parameters.

The `ebs-plugin` container should be CO-agnostic, therefore it should avoid directly interfacing with the Kubernetes API. Therefore we deploy the `ebs-plugin` alongside several [Kubernetes CSI sidecar containers](https://kubernetes-csi.github.io/docs/sidecar-containers.html). The `ebs-plugin` container implements the AWS-specific business logic for managing EBS volumes through direct interaction with AWS EC2 APIs or privileged OS system calls, while the Kubernetes CSI sidecar containers (like csi-provisioner, csi-attacher, and csi-resizer) handle the generic Kubernetes resource management by watching for changes to PVCs, VolumeAttachments, and other storage objects and translating them into standardized CSI RPCs. 

On Kubernetes, the EBS CSI Driver is deployed across two components:

- The EBS CSI controller component runs as a K8s Deployment called `ebs-csi-controller`. It watches K8s storage resources for changes, calls AWS EC2 APIs, and update those K8s resources once all associated EBS volumes reflect those changes. 
- The EBS CSI node component runs as a K8s DaemonSet called `ebs-csi-node` on every node. Each Node's Kubelet ensures EBS volumes appear as block devices or mounted filesystem directories inside the container workloads, by triggering CSI RPCs against the ebs-plugin's node service.

## Dynamically Provisioning an EBS Volume on Kubernetes

To illustrate the relationships between Kubernetes, EBS CSI controller/node components, and AWS, we can look at what happens when you dynamically provision a volume for a given stateful workload:

```mermaid
sequenceDiagram
    actor o as Operator
    participant k as Kubernetes
    participant c as ebs-csi-controller
    participant a as AWS EC2 API
    participant n as ebs-csi-node
    participant os as Node OS

    o->>k: Create PVC + Pod
    activate k

    note over k: Pod 'Pending' <br/> PVC 'Pending'
    k->>c: Notifies about PVC
    activate c
    c->>a: Ensure volume created
    a-->>c: 
    c-->>k: Create PV handle=<EBS volumeID> <br/> PVC and PV 'bound'
    deactivate c
    
    note over k: VolumeAttachment (VA) created
    k->>c: Notify about VolumeAttachment (VA)
    activate c
    c->>a: Ensure volume attached
    a-->>c: 
    c-->>k: Mark VA as Attached <br/> DevicePath='/dev/xvdaa'
    deactivate c
    
    note over k: Pod 'ContainerCreating'
    k->>n: Kubelet Triggers NodeStageVolume
    activate n
    opt If Filesystem 
    n->>os: Format/fsck device
    os-->>n: 
    end
    n->>os: Mount to '/var/lib/kubelet/<pod-id>/volumes/'
    os-->>n: 
    n-->>k: Kubelet update Node.volumesAttached
    deactivate n
    
    k->>n: Kubelet Triggers NodePublishVolume
    activate n
    n->>os: Bind-mount to container's 'mountPath'
    os-->>n: 
    n-->>k: Kubelet update Node.volumesInUse
    deactivate n
    
    note over k: Pod 'Running'
    k-->>o: Stateful workload running
    deactivate k
```

Zooming into the ebs-csi-controller pod, we can see how responsibility for ensuring the volume is attached is split between the Kubernetes CSI sidecars and the EBS CSI Driver Controller Service `ebs-plugin` container:

```mermaid
sequenceDiagram
    box ebs-csi-controller
    participant s as CSI Provisioner
    participant d as EBS Plugin
    end
    
    participant e as AWS API

    s->>d: CreateVolume RPC
    activate d
    
    d->>e: EC2 CreateVolume
    activate e
    
    e-->>d: 
    
    d->>e: Poll EC2 DescribeVolumes
    note over e: 1-3 Seconds
    e-->>d: Volume state == 'available'
    deactivate e
    
    d-->>s: Volume Response 
    deactivate d
```

## EBS CSI Driver Container Design Requirements

### Idempotency

All CSI calls should be idempotent. The CSI plugin must ensure that a CSI call with the same parameters will always return the same result. Examples:

- CreateVolume call must first check that the requested EBS volume has been already provisioned and return it if so. It should create a new volume only when such volume does not exist.
- ControllerPublish (=i.e. attach) does not do anything and returns &quot;success&quot; when given volume is already attached to requested node.
- DeleteVolume does not do anything and returns success when given volume is already deleted (i.e. it does not exist, we don&#39;t need to check that it had existed and someone really deleted it)

Note that it&#39;s task of the ebs-plugin to make these calls idempotent even if the related AWS API call is not.

### Timeouts

gRPC always passes a timeout together with a request. After this timeout, the gRPC client call actually returns. The server (i.e. ebs-plugin) can continue processing the call and finish the operation, however it has no means how to inform the client about the result.

Kubernetes sidecars will retry failed calls after exponential backoff. These sidecars rely on idempotency here - i.e. when ebs-plugin finished an operation after the client timed out, the ebs-plugin will get the same call again, and it should return success/error based on success/failure of the previous operation.

Example:

1. csi-attacher calls ControllerPublishVolume(vol1, nodeA) ), i.e. &quot;attach vol1 to nodeA&quot;.
2. ebs-plugin checks vol1 and sees it&#39;s not attached to nodeA yet. It calls EC2 AttachVolume(vol1, nodeA).
3. The attachment takes a long time, RPC times out.
4. csi-attacher sleeps for some time.
5. AWS finishes attaching of the volume.
6. csi-attacher re-issues ControllerPublishVolume(vol1, nodeA) again.
7. ebs-plugin checks vol1 and sees it is attached to nodeA and returns success immediately.

Note that there are some issues:

- Kubernetes can change its mind at any time. E.g. a user that wanted to run a pod on the node in the example got impatient so he deleted the pod at step 4. In this case csi-attacher will call ControllerUnpublishVolume(vol1, nodeA) to &quot;cancel&quot; the attachment request. It&#39;s up to the ebs-plugin to do the right thing - e.g. wait until the volume is attached and then issue detach() and wait until the volume is detached and \*then\* return from
- Note that Kubernetes may time out waiting for ControllerUnpublishVolume too. In this case, it will keep calling it until it gets confirmation from the driver that the volume has been detached (i.e. until ebs-plugin returns either success or non-timeout error) or it needs the volume attached to the node again (and it will call ControllerPublishVolume in that case).
- The same applies to NodeStage and NodePublish calls (&quot;mount device, mount volume&quot;). These are typically much faster than attach/detach, still they must be idempotent when it comes to timeouts.

In summary, always check that the required operation is already completed.

### Restarts

The CSI driver should survive its own crashes or reboots of the node where it runs. For the controller service, Kubernetes will either start a new driver on a different node or re-elect a new leader of stand-by drivers. For the node service, Kubernetes will start a new driver shortly.

The ideal CSI driver is stateless. After start, it should recover its state by observing the actual status of AWS (i.e. describe instances / volumes, list of unusable device names, etc.).

### No credentials on nodes

General security requirements we follow in Kubernetes is &quot;if a node gets compromised then the damage is limited to the node&quot;. Security conscious operators typically dedicate handful of nodes in Kubernetes cluster as &quot;infrastructure nodes&quot; and dedicate these nodes to run &quot;infrastructure pods&quot; only. Regular users can&#39;t run their pods there. CSI attacher and provisioner is an example of such &quot;infrastructure pod&quot; - it needs permission to create/delete any PV in Kubernetes and CSI driver running there needs credentials to create/delete volumes in AWS.

There should be a way how to run the CSI driver (=container) in &quot;node mode&quot; only. Such driver would then respond only to node service RPCs and it would not have any credentials to AWS (or very limited credentials, e.g. only to Describe things). Security conscious operators would deploy CSI driver in &quot;node only&quot; mode on all nodes where Kubernetes runs user containers.

Both the EBS CSI Driver Controller Deployment and Node Daemonset pods rely on the same `ebs-plugin` container. The difference is that in the controller pod we set the plugin's mode to `controller`, and in the node pods we set the plugin's mode to `node`.

Because the node pods set the ebs-plugin container to node mode, it won't respond to Controller Service RPCs (ControllerCreateVolume). 

Today, we also don't give any of the daemonset pods access to the EBS CSI Controller IAM Role by associating the node pods with a different ServiceAccount than the controller pods, because the Node Service RPCs like NodeStageVolume don't need to talk to the EC2 API. 

Finally, we ensure only the EBS CSI Controller pods have the higher-risk K8s RBAC for actions like patching PV and VolumeAttachment  resources. 

Therefore, security conscious customers can ensure that the EBS CSI Controller pods are only scheduled on extra hardened nodes. If an intruder gains access to any of the other nodes on the cluster they wouldn't be able to create/delete EBS volumes via the `ebs-csi-node` pod, or gain control over the cluster's storage resources (PVs, VAs).

### Assigning device names

On AWS, it&#39;s the client who [must assign device names](https://aws.amazon.com/premiumsupport/knowledge-center/ebs-stuck-attaching/) to volumes when calling AWS.AttachVolume. At the same time, AWS [imposes some restrictions on the device names](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html). Because of these restrictions, we must assign device names in a deterministic order, and maintain a cache of attempted device names that are likely unusable for a particular instance.  

## High level overview of CSI calls

### Identity Service RPC

#### GetPluginInfo

Blindly return:

```
  Name: ebs.csi.aws.com
  VendorVersion: 1.x.y
```

#### GetPluginCapabilities

Blindly return:

```
   Capabilities:
     - CONTROLLER_SERVICE
     - ACCESSIBILITY_CONSTRAINTS
     - ...
```

#### Probe

- This call is used by Kubernetes liveness probe to check that the driver is healthy. It&#39;s called every few seconds, so it should not do anything &quot;expensive&quot; or time-consuming.
- For the Controller Service, this probe will check that the driver has the required networking/auth by making an hourly DescribeAvailabilityZones call. 

### Controller Service RPC

#### CreateVolume

Checks that the requested volume was not created yet and creates it.

- Snapshot: if creating volume from snapshot, read the snapshot ID from request.

```mermaid
sequenceDiagram
    participant s as CSI Provisioner
    participant d as EBS Plugin
    participant e as AWS API

    s->>d: CreateVolume RPC
    activate d
    
    d->>d: Parse Request + Ensure Idempotency

    d->>e: EC2 CreateVolume
    activate e
    e-->>d: Get volumeID <br/> state == 'creating'
    

    d->>e: Poll EC2 DescribeVolumes
    note over e: 1-3 Seconds
    e-->>d: Volume state == 'available'
    deactivate e
    
    d-->>s: Return Volume Response 
    deactivate d
```

#### DeleteVolume

Checks if the required volume exists and is &quot;available&quot; (not attached anywhere) and deletes it if so. Returns success if the volume can&#39;t be found. Returns error if the volume is attached anywhere.

#### ControllerPublishVolume

- Checks that given volume is already attached to given node. Returns success if so.
- Checks that given volume is available (i.e. not attached to any other node) and returns error if it is attached.
- Chooses the right device name for the volume on the node (more on that below) and issues AttachVolume. TODO: this has complicated idempotency expectations. It cancels previously called ControllerUnpublishVolume that may be still in progress (i.e. AWS is still detaching the volume and Kubernetes now wants the volume to be attached back).

```mermaid
sequenceDiagram
    participant s as CSI Attacher
    participant d as EBS Plugin
    participant e as AWS API

    s->>d: ControllerPublishVolume RPC
    activate d
    
    d->>d: Parse Request + Ensure Idempotency

    d->>e: EC2 DescribeInstances
    
    e-->>d: Get instanceID + all device names

    d->>d: Assign likely unused device name
    
    d->>e: EC2 AttachVolume
    activate e
    e-->>d: Volume state == 'attaching'

    d->>e: Poll EC2 DescribeVolumes
    note over e: 2+ seconds
    e-->>d: Volume state == 'in-use'
    deactivate e
    
    d-->>s: Return device path 
    deactivate d
```

#### ControllerUnpublishVolume

Checks that given volume is not attached to given node. Returns success if so. Issues AWS.DetachVolume and marks the detached device name as free (more on that below). TODO: this has complicated idempotency expectations. It cancels previously called ControllerPublishVolume (i.e.AWS is still attaching the volume and Kubernetes now wants the volume to be detached).

#### ControllerExpandVolume

Checks that given volume is not expanded yet, calls EC2 ModifyVolume and ensures the modification enters the 'optimizing' state. 

Note: If ControllerModifyVolume is triggered within approximately 2 seconds of ControllerExpandVolume, they will share an EC2 ModifyVolume call. 

#### ControllerModifyVolume

Checks if volume needs modification, calls EC2 ModifyVolume, and ensures modification enters 'optimizing' state.

- If tags need to be created/modified, call EC2 CreateTags. 

Note: If ControllerExpandVolume is triggered within approximately 2 seconds of ControllerModifyVolume, they will share an EC2 ModifyVolume call.

#### ValidateVolumeCapabilities

Check whether access mode is supported for each capability

#### ControllerGetCapabilities

Blindly return:

```
  rpc:
    - CREATE\_DELETE\_VOLUME
    - PUBLISH\_UNPUBLISH\_VOLUME
    - ...
```

#### CreateSnapshot

Create a new snapshot from a source volume.

#### DeleteSnapshot

Deletes a snapshot.

#### ListSnapshots

List all EBS-CSI-Driver managed snapshots.

### Node Service RPC

#### NodeStageVolume

1. Find the device.
2. Check if it&#39;s unformatted (lsblk or blkid).
3. Format it if it&#39;s needed.
4. fsck it if it&#39;s already formatted + refuse to mount it on errors.
5. Mount it to given directory (with given mount options).

Steps 3 and 4 can take some time, so the driver must ensure idempotency somehow.

#### NodePublishVolume

Bind-mount the volume.

#### NodeUnstageVolume

Unmount the volume.

#### NodeUnpublishVolume

Unmount the volume.

#### NodeExpandVolume

If the attached volume has been formatted with a filesystem, resize the filesystem.

#### NodeGetVolumeStats

Returns the amount of available/total/used bytes and inodes for a given volume.

#### NodeGetInfo

Blindly return:

```
    NodeId: AWS InstanceID.
    AccessibleTopology: {"topology.ebs.csi.aws.com/zone": [availablility zone]}
```

#### NodeGetId

Return AWS InstanceID.

#### NodeGetCapabilities

Blindly return:

```
  rpc:
    - STAGE\_UNSTAGE\_VOLUME
```

## Coalescing ControllerExpandVolume & ControllerModifyVolume

### EC2 ModifyVolume and Request Coalescing

AWS exposes one unified ModifyVolume API to change the size, volume-type, IOPS, or throughput of your volume. AWS imposes a cooldown after a set number of volume modifications.

However, the CSI Specification exposes two separate RPCs that rely on ebs-plugin calling this EC2 ModifyVolume API: ControllerExpandVolume, for increasing volume size, and ControllerModifyVolume, for all other volume modifications. To avoid unnecessary `ModifyVolume` calls (potentially hitting the cooldown), we coalesce these separate expansion and modification requests by waiting for up to two seconds, and then perform one merged EC2 ModifyVolume API Call.

Here is an overview of what may happen when you patch a PVC's size and VolumeAttributesClassName at the same time:

```mermaid
sequenceDiagram
    participant o as Kubernetes
    box ebs-csi-controller Pod
        participant s as csi-resizer
        participant p as ebs-plugin <br/> (controller service)
    end
    participant a as AWS API
    participant n as ebs-plugin <br/> (node service)

    o->>s: Updated PVC VACName + PVC capacity
    activate o
    activate s
    
    s->>p: ControllerExpandVolume RPC
    activate p
    note over p: Wait up to 2s for other RPC
    s->>p:  ControllerModifyVolume RPC
    
    p->>p: Merge Expand + Modify Requests
    p->>a: EC2 CreateTags
    a-->>p: 
    p->>a: Check if volume needs modification via EC2 DescribeVolumes
    a-->>p: 
    p->>a: EC2 ModifyVolume
    activate a
    a-->>p: 
    p->>a: Poll EC2 DescribeVolumeModifications
    note over a: 1+ seconds
    a-->>p: Volume state == 'optimizing' || `completed`
    deactivate a
    
    p-->>s: EBS Volume Modified
    s-->>o: Emit VolumeModify Success Event
    p-->>s: EBS Volume Expanded
    deactivate p
    
    alt if Block Device
    s-->>o: Emit ExpandVolume Success Event
    else if Filesystem
    s-->>o: Mark PVC as FSResizeRequired
    deactivate s
    o->>n: Kubelet triggers NodeExpandVolume RPC
    activate n
    n->>n: Online resize of FS
    note over n: 1+ seconds
    n-->>o: Resize Success
    deactivate n
    deactivate o
    end
```

## Driver modes

Traditionally, you run the CSI controllers together with the EBS driver in the same Kubernetes cluster.
Though, in some scenarios you might want to run the CSI controllers (csi-provisioner, csi-attacher, etc.) together with the EBS controller service of this driver separately from the Kubernetes cluster it serves (while the EBS driver with an activated node service still runs inside the cluster).
This may not necessarily have to be in the same AWS region.
Also, the controllers may not necessarily have to run on an AWS EC2 instance.
To support these cases, the AWS EBS CSI driver plugin supports three modes:

- `all`: This is the standard/default mode that is used for the mentioned traditional scenario. It assumes that the CSI controllers run together with the EBS driver in the same AWS cluster. It starts both the controller and the node service of the driver.\
Example 1: `/bin/aws-ebs-csi-driver --extra-volume-tags=foo=bar`\
Example 2: `/bin/aws-ebs-csi-driver all --extra-volume-tags=foo=bar`

- `controller`: This will only start the controller service of the CSI driver. It enables use-cases as mentioned above, e.g., running the CSI controllers outside of the Kubernetes cluster they serve. Still, this mode assumes that it runs in the same AWS region on an AWS EC2 instance. If this is not true you may overwrite the region by specifying the `AWS_REGION` environment variable (if not specified the controller will try to use the AWS EC2 metadata service to look it up dynamically).\
Example 1: `/bin/aws-ebs-csi-driver controller --extra-volume-tags=foo=bar`\
Example 2: `AWS_REGION=us-west-1 /bin/aws-ebs-csi-driver controller --extra-volume-tags=foo=bar`\

- `node`: This will only start the node service of the CSI driver.\
Example: `/bin/aws-ebs-csi-driver node --endpoint=unix://...`
