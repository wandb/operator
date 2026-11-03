package helm

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func TestEnvSourcePostRenderer(t *testing.T) {
	input := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
        - name: api
          envFrom:
            - secretRef:
                name: custom-env
          env:
            - name: LITERAL
              value: "42"
            - name: SECRET
              valueFrom:
                secretKeyRef:
                  name: mysql
                  key: host
            - name: EMPTY
            - name: INVALID
              value: keep-invalid
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
      initContainers:
        - name: init
          env:
            - name: CONFIG
              valueFrom:
                configMapKeyRef:
                  name: config
                  key: host
---
apiVersion: example.test/v1
kind: Deployment
metadata:
  name: custom
spec:
  template:
    spec:
      containers:
        - name: api
          env:
            - name: UNRELATED
              value: unchanged
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  env: 'valueFrom: unchanged'
  numeric-string: "42"
`
	renderer := envSourcePostRenderer{}
	output, err := renderer.Run(bytes.NewBufferString(input))
	if err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) []unstructured.Unstructured {
		t.Helper()
		var result []unstructured.Unstructured
		decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			var obj unstructured.Unstructured
			if err := decoder.Decode(&obj); err == io.EOF {
				return result
			} else if err != nil {
				t.Fatal(err)
			}
			result = append(result, obj)
		}
	}
	before, after := decode([]byte(input)), decode(output.Bytes())
	if len(after) != len(before) {
		t.Fatalf("rendered %d documents, want %d", len(after), len(before))
	}
	for i := 1; i < len(before); i++ {
		if !reflect.DeepEqual(before[i].Object, after[i].Object) {
			t.Errorf("unrelated %s changed", before[i].GetKind())
		}
	}
	for field, want := range map[string]string{
		"containers": `[{"name":"api","envFrom":[{"secretRef":{"name":"custom-env"}}],"env":[
			{"name":"LITERAL","value":"42","valueFrom":null},
			{"name":"SECRET","value":null,"valueFrom":{"secretKeyRef":{"name":"mysql","key":"host"}}},
			{"name":"EMPTY","value":null,"valueFrom":null},
			{"name":"INVALID","value":"keep-invalid","valueFrom":{"fieldRef":{"fieldPath":"metadata.name"}}}
		]}]`,
		"initContainers": `[{"name":"init","env":[
			{"name":"CONFIG","value":null,"valueFrom":{"configMapKeyRef":{"name":"config","key":"host"}}}
		]}]`,
	} {
		var expected interface{}
		if err := json.Unmarshal([]byte(want), &expected); err != nil {
			t.Fatal(err)
		}
		actual, _, err := unstructured.NestedSlice(after[0].Object, "spec", "template", "spec", field)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s = %#v, want %#v", field, actual, expected)
		}
	}
	repeated, err := renderer.Run(bytes.NewBuffer(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), repeated.Bytes()) {
		t.Error("repeated rendering changed the manifests")
	}
}

func TestEnvSourcePostRendererRejectsInvalidYAML(t *testing.T) {
	if _, err := (envSourcePostRenderer{}).Run(bytes.NewBufferString("invalid: [")); err == nil {
		t.Fatal("expected a rendering error")
	}
}
