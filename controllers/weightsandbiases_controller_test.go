package controllers

import (
	"context"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer/deployerfakes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"reflect"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestWeightsAndBiasesReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client         client.Client
		IsAirgapped    bool
		DeployerClient deployer.DeployerInterface
		Scheme         *runtime.Scheme
		Recorder       record.EventRecorder
	}
	type args struct {
		ctx context.Context
		req controllerruntime.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    controllerruntime.Result
		wantErr bool
	}{
		{
			name: "Test Reconcile",
			fields: fields{
				Client:         k8sClient,
				IsAirgapped:    false,
				DeployerClient: &deployerfakes.FakeDeployerInterface{},
				Scheme:         scheme.Scheme,
				Recorder:       record.NewFakeRecorder(10),
			},
			args: args{
				ctx: context.Background(),
				req: controllerruntime.Request{},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &WeightsAndBiasesReconciler{
				Client:         tt.fields.Client,
				IsAirgapped:    tt.fields.IsAirgapped,
				DeployerClient: tt.fields.DeployerClient,
				Scheme:         tt.fields.Scheme,
				Recorder:       tt.fields.Recorder,
			}
			got, err := r.Reconcile(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reconcile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
