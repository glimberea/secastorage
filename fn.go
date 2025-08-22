package main

import (
	"context"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/ionos-cloud/provider-upjet-ionoscloud/apis/compute/v1alpha1"
	"github.com/ionos-cloud/sdk-go-bundle/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/pkg/errors"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)
	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composite resource from %T", req))
		return rsp, nil
	}
	workspace, err := xr.Resource.GetString("spec.workspace")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.workspace field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	region, err := xr.Resource.GetString("spec.region")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.region field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	tenant, err := xr.Resource.GetString("spec.tenant")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.tenant field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	image, err := xr.Resource.GetString("spec.image")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.image field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	sizeGB, err := xr.Resource.GetInteger("spec.sizeGB")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.sizeGB field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	// todo - convert sku to ionos storage
	// _, err := xr.Resource.GetString("spec.sku")
	// if err != nil {
	// 	response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.sku field of %s", xr.Resource.GetKind()))
	// 	return rsp, nil
	// }

	name, err := xr.Resource.GetString("spec.name")
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot read spec.name field of %s", xr.Resource.GetKind()))
		return rsp, nil
	}
	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired resources from %T", req))
		return rsp, nil
	}
	// todo - check if datacenter already exists. If it does, get it's ID and use it when creating the volume.
	datacenterName := workspace + "-datacenter"
	f.log.Info("Creating datacenter", "generatedName", datacenterName)
	_ = v1alpha1.AddToScheme(composed.Scheme)
	datacenter := &v1alpha1.Datacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: datacenterName,
			Labels: map[string]string{
				"ionos-cloud-datacenter-name": datacenterName,
				"ionos-cloud-dc":              workspace,
				"ionos-cloud-region":          region,
				"ionos-cloud-tenant":          tenant,
			},
		},
		Spec: v1alpha1.DatacenterSpec{
			ForProvider: v1alpha1.DatacenterParameters{
				Description: shared.ToPtr("Datacenter for " + workspace),
				Location:    shared.ToPtr(region),
				Name:        shared.ToPtr(datacenterName),
			},
		},
	}
	cd, err := composed.From(datacenter)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert %T to %T", datacenter, &composed.Unstructured{}))
		return rsp, nil
	}
	desired[resource.Name("xservers-"+datacenterName)] = &resource.DesiredComposed{Resource: cd}
	f.log.Info("Creating volume", "name", workspace+"_volume")
	vol := v1alpha1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: workspace + "_volume",
		},
		Spec: v1alpha1.VolumeSpec{
			ForProvider: v1alpha1.VolumeParameters_2{
				DatacenterIDSelector: &v1.Selector{
					MatchLabels: map[string]string{
						"ionos-cloud-dc": workspace,
					},
				},
				// ServerIDSelector: &v1.Selector{
				// 	MatchLabels: map[string]string{
				// 		"ionos-cloud-workspace": workspace,
				// 	},
				// },
				DiskType:      shared.ToPtr("SSD"),
				ImageName:     shared.ToPtr(image),
				ImagePassword: shared.ToPtr("thisisnotapassword"),
				Name:          shared.ToPtr(name),
				Size:          shared.ToPtr(float64(sizeGB)),
			},
		},
	}
	cd, err = composed.From(&vol)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert %T to %T", vol, &composed.Unstructured{}))
		return rsp, nil
	}
	desired[resource.Name("xservers-"+name)] = &resource.DesiredComposed{Resource: cd}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources in %T", rsp))
		return rsp, nil
	}
	// You can set a custom status condition on the claim. This allows you to
	// communicate with the user. See the link below for status condition
	// guidance.
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	response.ConditionTrue(rsp, "FunctionSuccess", "Success").
		TargetCompositeAndClaim()

	return rsp, nil
}
