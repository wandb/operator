package helm

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// ObjectSelector represents a boolean expression for selecting objects.
type ObjectSelector = func(runtime.Object) bool

// ObjectEditor represents a method for editing objects.
type ObjectEditor = func(runtime.Object) error

// Template represents a Helm template and provides methods to query and edit
// objects that are loaded from the after rendering it.
type Template interface {
	// Namespace returns namespace of the template. Builder sets this value and
	// it can not be changed.
	Namespace() string

	// ReleaseName returns release name of the template. Builder sets this value
	// and it can not be changed.
	ReleaseName() string

	// Warnings returns the list of warnings that occurred while rendering this
	// template. Any error that can be ignored is a warning.
	Warnings() []error

	// Objects returns the list of all available objects.
	Objects() []runtime.Object

	// GetObjects returns all objects that match the selector.
	GetObjects(selector ObjectSelector) ([]runtime.Object, error)

	// AddObject adds a new object to the template. Implementing this method is
	// optional. When the operation is not supported it returns an error.
	AddObject(object runtime.Object) error

	// DeleteObjects deletes all objects that match the selector and returns the
	// number of deleted objects. Implementing this method is optional. When the
	// operation is not supported it returns an error.
	DeleteObjects(selector ObjectSelector) (int, error)

	// ReplaceObject replaces the first object that matches the selector with
	// the new object. Implementing this method is optional. When the operation
	// is not supported it returns an error.
	ReplaceObject(selector ObjectSelector, object runtime.Object) (runtime.Object, error)

	// EditObjects edits all objects that the editor can handle in place and
	// returns the number of edited objects. Implementing this method is
	// optional. When the operation is not supported it returns an error.
	EditObjects(editor ObjectEditor) (int, error)
}


type mutableTemplate struct {
	releaseName string
	namespace   string
	objects     []runtime.Object
	warnings    []error
}

func newMutableTemplate(releaseName, namespace string) *mutableTemplate {
	template := &mutableTemplate{
		releaseName: releaseName,
		namespace:   namespace,
		objects:     []runtime.Object{},
		warnings:    []error{},
	}
	// template.query = newQuery(template)

	return template
}

func (t *mutableTemplate) Namespace() string {
	return t.namespace
}

func (t *mutableTemplate) ReleaseName() string {
	return t.releaseName
}

func (t *mutableTemplate) Warnings() []error {
	return t.warnings
}

func (t *mutableTemplate) Objects() []runtime.Object {
	return t.objects
}

func (t *mutableTemplate) GetObjects(selector ObjectSelector) ([]runtime.Object, error) {
	result := []runtime.Object{}

	for i := 0; i < len(t.objects); i++ {
		if selector(t.objects[i]) {
			result = append(result, t.objects[i])
		}
	}

	return result, nil
}

func (t *mutableTemplate) AddObject(object runtime.Object) error {
	t.objects = append(t.objects, object)

	return nil
}

func (t *mutableTemplate) DeleteObjects(selector ObjectSelector) (int, error) {
	count := 0

	for i := 0; i < len(t.objects); i++ {
		if selector(t.objects[i]) {
			t.objects = append(t.objects[:i], t.objects[i+1:]...)
			count++
			i--
		}
	}

	return count, nil
}

func (t *mutableTemplate) ReplaceObject(selector ObjectSelector, object runtime.Object) (runtime.Object, error) {
	for i := 0; i < len(t.objects); i++ {
		if selector(t.objects[i]) {
			old := t.objects[i]
			t.objects[i] = object

			return old, nil
		}
	}

	return nil, nil
}

func (t *mutableTemplate) EditObjects(editor ObjectEditor) (int, error) {
	count := 0

	for i := 0; i < len(t.objects); i++ {
		err := editor(t.objects[i])
		if err != nil {
			if IsTypeMistmatchError(err) {
				continue
			}

			return count, err
		}
		count++
	}

	return count, nil
}


type typeMistmatchError struct {
	expected interface{}
	observed interface{}
}

func (e *typeMistmatchError) Error() string {
	return fmt.Sprintf("expected %T, got %T", e.expected, e.observed)
}

func NewTypeMistmatchError(expected, observed interface{}) error {
	return &typeMistmatchError{
		expected: expected,
		observed: observed,
	}
}

// IsTypeMistmatchError returns true if the error is raised because of type mistmatch.
func IsTypeMistmatchError(err error) bool {
	_, ok := err.(*typeMistmatchError)
	return ok
}

