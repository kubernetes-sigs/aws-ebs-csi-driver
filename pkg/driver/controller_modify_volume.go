package driver

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	ModificationKeyVolumeType = "volumeType"

	ModificationKeyIOPS = "iops"

	ModificationKeyThroughput = "throughput"

	modifyVolumeRequestHandlerTimeout = 2 * time.Second
)

type modifyVolumeRequest struct {
	newSize           int64
	modifyDiskOptions cloud.ModifyDiskOptions
	// Channel for sending the response to the request caller
	responseChan chan modifyVolumeResponse
}

type modifyVolumeResponse struct {
	volumeSize int64
	err        error
}

type modifyVolumeRequestHandler struct {
	volumeID string
	// Merged request from the requests that have been accepted for the volume
	mergedRequest *modifyVolumeRequest
	// Channel for sending requests to the goroutine for the volume
	requestChan chan *modifyVolumeRequest
}

type modifyVolumeManager struct {
	// Map of volume ID to modifyVolumeRequestHandler
	requestHandlerMap sync.Map
}

func newModifyVolumeManager() *modifyVolumeManager {
	return &modifyVolumeManager{
		requestHandlerMap: sync.Map{},
	}
}

func newModifyVolumeRequestHandler(volumeID string, request *modifyVolumeRequest) modifyVolumeRequestHandler {
	requestChan := make(chan *modifyVolumeRequest)
	return modifyVolumeRequestHandler{
		requestChan:   requestChan,
		mergedRequest: request,
		volumeID:      volumeID,
	}
}

// This function validates the new request against the merged request for the volume.
// If the new request has a volume property that's already included in the merged request and its value is different from that in the merged request,
// this function will return an error and the new request will be rejected.
func (h *modifyVolumeRequestHandler) validateModifyVolumeRequest(r *modifyVolumeRequest) error {
	if r.newSize != 0 && h.mergedRequest.newSize != 0 && r.newSize != h.mergedRequest.newSize {
		return fmt.Errorf("Different size was requested by a previous request. Current: %d, Requested: %d", h.mergedRequest.newSize, r.newSize)
	}
	if r.modifyDiskOptions.IOPS != 0 && h.mergedRequest.modifyDiskOptions.IOPS != 0 && r.modifyDiskOptions.IOPS != h.mergedRequest.modifyDiskOptions.IOPS {
		return fmt.Errorf("Different IOPS was requested by a previous request. Current: %d, Requested: %d", h.mergedRequest.modifyDiskOptions.IOPS, r.modifyDiskOptions.IOPS)
	}
	if r.modifyDiskOptions.Throughput != 0 && h.mergedRequest.modifyDiskOptions.Throughput != 0 && r.modifyDiskOptions.Throughput != h.mergedRequest.modifyDiskOptions.Throughput {
		return fmt.Errorf("Different throughput was requested by a previous request. Current: %d, Requested: %d", h.mergedRequest.modifyDiskOptions.Throughput, r.modifyDiskOptions.Throughput)
	}
	if r.modifyDiskOptions.VolumeType != "" && h.mergedRequest.modifyDiskOptions.VolumeType != "" && r.modifyDiskOptions.VolumeType != h.mergedRequest.modifyDiskOptions.VolumeType {
		return fmt.Errorf("Different volume type was requested by a previous request. Current: %s, Requested: %s", h.mergedRequest.modifyDiskOptions.VolumeType, r.modifyDiskOptions.VolumeType)
	}
	return nil
}

func (h *modifyVolumeRequestHandler) mergeModifyVolumeRequest(r *modifyVolumeRequest) {
	if r.newSize != 0 {
		h.mergedRequest.newSize = r.newSize
	}
	if r.modifyDiskOptions.IOPS != 0 {
		h.mergedRequest.modifyDiskOptions.IOPS = r.modifyDiskOptions.IOPS
	}
	if r.modifyDiskOptions.Throughput != 0 {
		h.mergedRequest.modifyDiskOptions.Throughput = r.modifyDiskOptions.Throughput
	}
	if r.modifyDiskOptions.VolumeType != "" {
		h.mergedRequest.modifyDiskOptions.VolumeType = r.modifyDiskOptions.VolumeType
	}
}

// processModifyVolumeRequests method starts its execution with a timer that has modifyVolumeRequestHandlerTimeout as its timeout value.
// When the Timer times out, it calls the ec2 API to perform the volume modification. processModifyVolumeRequests method sends back the response of
// the ec2 API call to the CSI Driver main thread via response channels.
// This method receives requests from CSI driver main thread via the request channel. When a new request is received from the request channel, we first
// validate the new request. If the new request is acceptable, it will be merged with the existing request for the volume.
func (d *controllerService) processModifyVolumeRequests(h *modifyVolumeRequestHandler, responseChans []chan modifyVolumeResponse) {
	klog.V(4).InfoS("Start processing ModifyVolumeRequest for ", "volume ID", h.volumeID)
	process := func(req *modifyVolumeRequest) {
		if err := h.validateModifyVolumeRequest(req); err != nil {
			req.responseChan <- modifyVolumeResponse{err: err}
		} else {
			h.mergeModifyVolumeRequest(req)
			responseChans = append(responseChans, req.responseChan)
		}
	}

	for {
		select {
		case req := <-h.requestChan:
			process(req)
		case <-time.After(modifyVolumeRequestHandlerTimeout):
			d.modifyVolumeManager.requestHandlerMap.Delete(h.volumeID)
			// At this point, no new requests can come in on the request channel because it has been removed from the map
			// However, the request channel may still have requests waiting on it
			// Thus, process any requests still waiting in the channel
			for loop := true; loop; {
				select {
				case req := <-h.requestChan:
					process(req)
				default:
					loop = false
				}
			}
			actualSizeGiB, err := d.executeModifyVolumeRequest(h.volumeID, h.mergedRequest)
			for _, c := range responseChans {
				select {
				case c <- modifyVolumeResponse{volumeSize: actualSizeGiB, err: err}:
				default:
					klog.V(6).InfoS("Ignoring response channel because it has no receiver", "volumeID", h.volumeID)
				}
			}
			return
		}
	}
}

// When a new request comes in, we look up requestHandlerMap using the volume ID of the request.
// If there's no ModifyVolumeRequestHandler for the volume, meaning that there’s no inflight requests for the volume, we will start a goroutine
// for the volume calling processModifyVolumeRequests method, and ModifyVolumeRequestHandler for the volume will be added to requestHandlerMap.
// If there’s ModifyVolumeRequestHandler for the volume, meaning that there is inflight request(s) for the volume, we will send the new request
// to the goroutine for the volume via the receiving channel.
// Note that each volume with inflight requests has their own goroutine which follows timeout schedule of their own.
func (d *controllerService) addModifyVolumeRequest(volumeID string, r *modifyVolumeRequest) {
	requestHandler := newModifyVolumeRequestHandler(volumeID, r)
	handler, loaded := d.modifyVolumeManager.requestHandlerMap.LoadOrStore(volumeID, requestHandler)
	if loaded {
		h := handler.(modifyVolumeRequestHandler)
		h.requestChan <- r
	} else {
		responseChans := []chan modifyVolumeResponse{r.responseChan}
		go d.processModifyVolumeRequests(&requestHandler, responseChans)
	}
}

func (d *controllerService) executeModifyVolumeRequest(volumeID string, req *modifyVolumeRequest) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	actualSizeGiB, err := d.cloud.ResizeOrModifyDisk(ctx, volumeID, req.newSize, &req.modifyDiskOptions)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "Could not modify volume %q: %v", volumeID, err)
	} else {
		return actualSizeGiB, nil
	}
}

func (d *controllerService) GetCSIDriverModificationCapability(
	_ context.Context,
	_ *rpc.GetCSIDriverModificationCapabilityRequest,
) (*rpc.GetCSIDriverModificationCapabilityResponse, error) {
	return &rpc.GetCSIDriverModificationCapabilityResponse{}, nil
}

func (d *controllerService) ModifyVolumeProperties(
	ctx context.Context,
	req *rpc.ModifyVolumePropertiesRequest,
) (*rpc.ModifyVolumePropertiesResponse, error) {
	klog.V(4).InfoS("ModifyVolumeProperties called", "req", req)
	if err := validateModifyVolumePropertiesRequest(req); err != nil {
		return nil, err
	}

	name := req.GetName()
	modifyOptions := cloud.ModifyDiskOptions{}
	for key, value := range req.GetParameters() {
		switch key {
		case ModificationKeyIOPS:
			iops, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse IOPS: %q", value)
			}
			modifyOptions.IOPS = iops
		case ModificationKeyThroughput:
			throughput, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse throughput: %q", value)
			}
			modifyOptions.Throughput = throughput
		case ModificationKeyVolumeType:
			modifyOptions.VolumeType = value
		}
	}

	responseChan := make(chan modifyVolumeResponse)
	request := modifyVolumeRequest{
		modifyDiskOptions: modifyOptions,
		responseChan:      responseChan,
	}

	// Intentionally not pass in context as we deal with context locally in this method
	d.addModifyVolumeRequest(name, &request) //nolint:contextcheck

	select {
	case response := <-responseChan:
		if response.err != nil {
			return nil, status.Errorf(codes.Internal, "Could not modify volume %q: %v", name, response.err)
		}
	case <-ctx.Done():
		return nil, status.Errorf(codes.Internal, "Could not modify volume %q: context cancelled", name)
	}

	return &rpc.ModifyVolumePropertiesResponse{}, nil
}

func validateModifyVolumePropertiesRequest(req *rpc.ModifyVolumePropertiesRequest) error {
	name := req.GetName()
	if name == "" {
		return status.Error(codes.InvalidArgument, "Volume name not provided")
	}
	return nil
}
