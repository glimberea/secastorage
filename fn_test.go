package main

import (
	"context"
	"testing"

	crv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ionos-cloud/provider-upjet-ionoscloud/apis/compute/v1alpha1"
	"github.com/ionos-cloud/sdk-go-bundle/shared"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
)

func TestRunFunction(t *testing.T) {
	_ = v1alpha1.AddToScheme(composed.Scheme)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "The function should successfully create a datacenter and a volume.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "success"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XSeCaStorage",
								"metadata": {
									"name": "test-xr"
								},
								"spec": {
									"workspace": "test-ws",
									"region": "de/txl",
									"tenant": "test-tenant",
									"image": "test-image",
									"sizeGB": 50,
									"name": "test-volume"
								}
							}`),
						},
					},
				},
			},
			want: want{
				rsp: func() *fnv1.RunFunctionResponse {
					datacenter := &v1alpha1.Datacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ws-datacenter",
							Labels: map[string]string{
								"ionos-cloud-datacenter-name": "test-ws-datacenter",
								"ionos-cloud-dc":              "test-ws",
								"ionos-cloud-region":          "de/txl",
								"ionos-cloud-tenant":          "test-tenant",
							},
						},
						Spec: v1alpha1.DatacenterSpec{
							ForProvider: v1alpha1.DatacenterParameters{
								Description: shared.ToPtr("Datacenter for test-ws"),
								Location:    shared.ToPtr("de/txl"),
								Name:        shared.ToPtr("test-ws-datacenter"),
							},
						},
					}
					cdDatacenter, err := composed.From(datacenter)
					if err != nil {
						t.Fatal(err)
					}

					vol := &v1alpha1.Volume{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ws_volume",
						},
						Spec: v1alpha1.VolumeSpec{
							ForProvider: v1alpha1.VolumeParameters_2{
								DatacenterIDSelector: &crv1.Selector{
									MatchLabels: map[string]string{
										"ionos-cloud-dc": "test-ws",
									},
								},
								DiskType:      shared.ToPtr("SSD"),
								ImageName:     shared.ToPtr("test-image"),
								ImagePassword: shared.ToPtr("thisisnotapassword"),
								Name:          shared.ToPtr("test-volume"),
								Size:          shared.ToPtr(float64(50)),
							},
						},
					}
					cdVolume, err := composed.From(vol)
					if err != nil {
						t.Fatal(err)
					}
					volResult, err := cdVolume.MarshalJSON()
					if err != nil {
						t.Fatal(err)
					}

					dcResult, err := cdDatacenter.MarshalJSON()
					if err != nil {
						t.Fatal(err)
					}
					rsp := response.To(&fnv1.RunFunctionRequest{Meta: &fnv1.RequestMeta{Tag: "success"}}, response.DefaultTTL)
					rsp.Desired = &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"xservers-test-volume": {
								Resource: resource.MustStructJSON(string(volResult)),
							},
							"xservers-test-ws-datacenter": {
								Resource: resource.MustStructJSON(string(dcResult)),
							},
						},
					}
					response.ConditionTrue(rsp, "FunctionSuccess", "Success").TargetCompositeAndClaim()
					return rsp
				}(),
			},
		},
		"FatalMissingWorkspace": {
			reason: "The function should return a fatal response if spec.workspace is missing.",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "fatal"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XSeCaStorage"
							}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "fatal", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot read spec.workspace field of XSeCaStorage: spec: no such field",
							Target:   ptr.To(fnv1.Target_TARGET_COMPOSITE),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
