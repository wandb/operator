//go:build !ignore_autogenerated

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightsAndBiases) DeepCopyInto(out *WeightsAndBiases) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightsAndBiases.
func (in *WeightsAndBiases) DeepCopy() *WeightsAndBiases {
	if in == nil {
		return nil
	}
	out := new(WeightsAndBiases)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WeightsAndBiases) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightsAndBiasesList) DeepCopyInto(out *WeightsAndBiasesList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WeightsAndBiases, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightsAndBiasesList.
func (in *WeightsAndBiasesList) DeepCopy() *WeightsAndBiasesList {
	if in == nil {
		return nil
	}
	out := new(WeightsAndBiasesList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WeightsAndBiasesList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightsAndBiasesSpec) DeepCopyInto(out *WeightsAndBiasesSpec) {
	*out = *in
	in.Chart.DeepCopyInto(&out.Chart)
	in.Values.DeepCopyInto(&out.Values)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightsAndBiasesSpec.
func (in *WeightsAndBiasesSpec) DeepCopy() *WeightsAndBiasesSpec {
	if in == nil {
		return nil
	}
	out := new(WeightsAndBiasesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WeightsAndBiasesStatus) DeepCopyInto(out *WeightsAndBiasesStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WeightsAndBiasesStatus.
func (in *WeightsAndBiasesStatus) DeepCopy() *WeightsAndBiasesStatus {
	if in == nil {
		return nil
	}
	out := new(WeightsAndBiasesStatus)
	in.DeepCopyInto(out)
	return out
}
