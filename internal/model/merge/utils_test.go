package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeMap(t *testing.T) {
	t.Run("merges maps with actual overriding defaults", func(t *testing.T) {
		defaults := map[string]string{
			"key1": "default1",
			"key2": "default2",
			"key3": "default3",
		}
		actual := map[string]string{
			"key2": "actual2",
			"key4": "actual4",
		}

		result := mergeMap(actual, defaults)

		expected := map[string]string{
			"key1": "default1",
			"key2": "actual2",
			"key3": "default3",
			"key4": "actual4",
		}
		assert.Equal(t, expected, result)
	})

	t.Run("returns defaults when actual is empty", func(t *testing.T) {
		defaults := map[string]int{
			"a": 1,
			"b": 2,
		}
		actual := map[string]int{}

		result := mergeMap(actual, defaults)

		assert.Equal(t, defaults, result)
	})

	t.Run("returns actual when defaults is empty", func(t *testing.T) {
		defaults := map[string]int{}
		actual := map[string]int{
			"x": 10,
			"y": 20,
		}

		result := mergeMap(actual, defaults)

		assert.Equal(t, actual, result)
	})

	t.Run("returns empty map when both are empty", func(t *testing.T) {
		defaults := map[string]string{}
		actual := map[string]string{}

		result := mergeMap(actual, defaults)

		assert.Empty(t, result)
	})

	t.Run("handles nil maps", func(t *testing.T) {
		var defaults map[string]string
		var actual map[string]string

		result := mergeMap(actual, defaults)

		assert.Empty(t, result)
	})

	t.Run("handles nil actual with non-nil defaults", func(t *testing.T) {
		defaults := map[string]string{
			"key1": "value1",
		}
		var actual map[string]string

		result := mergeMap(actual, defaults)

		assert.Equal(t, defaults, result)
	})

	t.Run("handles nil defaults with non-nil actual", func(t *testing.T) {
		var defaults map[string]string
		actual := map[string]string{
			"key1": "value1",
		}

		result := mergeMap(actual, defaults)

		assert.Equal(t, actual, result)
	})

	t.Run("works with integer keys", func(t *testing.T) {
		defaults := map[int]string{
			1: "one",
			2: "two",
		}
		actual := map[int]string{
			2: "TWO",
			3: "three",
		}

		result := mergeMap(actual, defaults)

		expected := map[int]string{
			1: "one",
			2: "TWO",
			3: "three",
		}
		assert.Equal(t, expected, result)
	})

	t.Run("works with struct values", func(t *testing.T) {
		type Value struct {
			Name string
			Age  int
		}

		defaults := map[string]Value{
			"alice": {Name: "Alice", Age: 30},
			"bob":   {Name: "Bob", Age: 25},
		}
		actual := map[string]Value{
			"bob":     {Name: "Bob", Age: 26},
			"charlie": {Name: "Charlie", Age: 35},
		}

		result := mergeMap(actual, defaults)

		expected := map[string]Value{
			"alice":   {Name: "Alice", Age: 30},
			"bob":     {Name: "Bob", Age: 26},
			"charlie": {Name: "Charlie", Age: 35},
		}
		assert.Equal(t, expected, result)
	})

	t.Run("does not modify input maps", func(t *testing.T) {
		defaults := map[string]string{
			"key1": "default1",
		}
		actual := map[string]string{
			"key2": "actual2",
		}

		defaultsCopy := map[string]string{
			"key1": "default1",
		}
		actualCopy := map[string]string{
			"key2": "actual2",
		}

		_ = mergeMap(actual, defaults)

		assert.Equal(t, defaultsCopy, defaults)
		assert.Equal(t, actualCopy, actual)
	})
}
