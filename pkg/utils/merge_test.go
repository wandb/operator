package utils

import (
	"reflect"
	"testing"
)

func TestMergeNullValues(t *testing.T) {
	tests := []struct {
		name string
		src  interface{}
		dst  interface{}
		want interface{}
	}{
		{name: "both null"},
		{name: "null overrides scalar", dst: "default"},
		{name: "null overrides map", dst: map[string]interface{}{"enabled": true}},
		{name: "null overrides slice", dst: []interface{}{"default"}},
		{name: "scalar overrides null", src: "configured", want: "configured"},
		{name: "false overrides null", src: false, want: false},
		{name: "zero overrides null", src: 0, want: 0},
		{name: "empty string overrides null", src: "", want: ""},
		{name: "empty map overrides null", src: map[string]interface{}{}, want: map[string]interface{}{}},
		{name: "empty slice overrides null", src: []interface{}{}, want: []interface{}{}},
		{name: "null overrides empty map", dst: map[string]interface{}{}},
		{name: "null overrides empty slice", dst: []interface{}{}},
		{name: "map overrides null", src: map[string]interface{}{"enabled": true}, want: map[string]interface{}{"enabled": true}},
		{name: "slice overrides null", src: []interface{}{"configured"}, want: []interface{}{"configured"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Merge(tt.src, tt.dst)
			if err != nil {
				t.Fatalf("Merge() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Merge() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMergeMapStringNullCPULimits(t *testing.T) {
	tests := []struct {
		name string
		src  map[string]interface{}
		dst  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "duplicate null CPU limits",
			src:  map[string]interface{}{"cpu": nil, "memory": "8Gi"},
			dst:  map[string]interface{}{"cpu": nil, "memory": "4Gi"},
			want: map[string]interface{}{"cpu": nil, "memory": "8Gi"},
		},
		{
			name: "null CPU limit overrides default",
			src:  map[string]interface{}{"cpu": nil},
			dst:  map[string]interface{}{"cpu": "2", "memory": "8Gi"},
			want: map[string]interface{}{"cpu": nil, "memory": "8Gi"},
		},
		{
			name: "configured CPU limit overrides null",
			src:  map[string]interface{}{"cpu": "2"},
			dst:  map[string]interface{}{"cpu": nil},
			want: map[string]interface{}{"cpu": "2"},
		},
		{
			name: "null CPU limit is copied when missing",
			src:  map[string]interface{}{"memory": "8Gi"},
			dst:  map[string]interface{}{"cpu": nil},
			want: map[string]interface{}{"cpu": nil, "memory": "8Gi"},
		},
		{
			name: "null CPU limit is retained when absent from defaults",
			src:  map[string]interface{}{"cpu": nil},
			dst:  map[string]interface{}{"memory": "8Gi"},
			want: map[string]interface{}{"cpu": nil, "memory": "8Gi"},
		},
	}

	values := func(limits map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"app": map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": limits,
				},
			},
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeMapString(values(tt.src), values(tt.dst))
			if err != nil {
				t.Fatalf("MergeMapString() error = %v", err)
			}
			if want := values(tt.want); !reflect.DeepEqual(got, want) {
				t.Errorf("MergeMapString() = %#v, want %#v", got, want)
			}
		})
	}
}
