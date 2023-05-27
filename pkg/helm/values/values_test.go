package values

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Values", func() {
	It("is empty after initialization", func() {
		Expect(Values{}).To(BeEmpty())
	})

	It("works like a map", func() {
		v := Values{
			"a": true,
			"b": "B",
		}

		Expect(v).To(HaveLen(2))
		Expect(v).To(HaveKeyWithValue("a", true))
		Expect(v).To(HaveKeyWithValue("b", "B"))
		Expect(v).NotTo(HaveKey("x"))
	})

	Describe("GetValue", func() {
		v := Values{
			"a": "A",
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": "D",
				},
			},
			"e": map[string]interface{}{
				"f": "F",
			},
		}

		It("returns the whole object when key is empty", func() {
			val, err := v.GetValue("")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(v))
		})

		It("returns the value of existing dot-separated keys", func() {
			val, err := v.GetValue("a")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("A"))

			val, err = v.GetValue("b.c.d")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("D"))

			val, err = v.GetValue("e.f")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("F"))
		})

		It("returns error for nested element of leaf nodes", func() {
			_, err := v.GetValue("a.b")
			Expect(err).To(HaveOccurred())
		})

		It("returns nil for missing elements of non-leaf nodes", func() {
			val, err := v.GetValue("b.x")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(BeNil())
		})

		It("returns error for nested missing element of non-leaf nodes", func() {
			_, err := v.GetValue("b.x.y")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SetValue", func() {
		v := Values{
			"a": "A",
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": "D",
				},
			},
		}

		It("overrides the value of existing dot-separated keys", func() {
			Expect(v.SetValue("a", "A2")).To(Succeed())
			Expect(v.SetValue("b.c.d", "D2")).To(Succeed())

			val, err := v.GetValue("a")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("A2"))

			val, err = v.GetValue("b.c.d")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("D2"))
		})

		It("adds the value of missing dot-separated keys", func() {
			Expect(v.SetValue("x", "X1")).To(Succeed())
			Expect(v.SetValue("b.x", "X2")).To(Succeed())
			Expect(v.SetValue("i.j.k", "K")).To(Succeed())

			val, err := v.GetValue("x")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("X1"))

			val, err = v.GetValue("b.x")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("X2"))

			val, err = v.GetValue("i.j.k")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("K"))
		})

		It("returns error when the key is empty", func() {
			Expect(v.SetValue("", Values{})).NotTo(Succeed())
		})

		It("returns error for setting nested element of leaf nodes", func() {
			Expect(v.SetValue("a.b", "B")).NotTo(Succeed())
		})
	})

	Describe("GetString", func() {
		v := Values{
			"a": "A",
			"b": true,
		}

		It("retruns the value of existing string-valued keys", func() {
			Expect(v.GetString("a")).To(Equal("A"))
		})

		It("returns the formatted value of existing non-string-valued keys", func() {
			Expect(v.GetString("b")).To(Equal("true"))
		})

		It("returns empty string for missing keys", func() {
			Expect(v.GetString("c")).To(Equal(""))
		})

		It("returns the default value for missing keys", func() {
			Expect(v.GetString("d", "D")).To(Equal("D"))
		})
	})

	Describe("GetBool", func() {
		v := Values{
			"a": "A",
			"b": true,
		}

		It("retruns the value of existing boolean-valued keys", func() {
			Expect(v.GetBool("b")).To(Equal(true))
		})

		It("returns false for existing non-boolean-valued keys", func() {
			Expect(v.GetBool("a")).To(Equal(false))
		})

		It("returns false for missing keys", func() {
			Expect(v.GetBool("c")).To(Equal(false))
		})

		It("returns the default value for missing keys", func() {
			Expect(v.GetBool("d", true)).To(Equal(true))
		})
	})

	Describe("Merge", func() {
		It("can deep merge another Values", func() {
			dst := Values{
				/* scalar */
				"a": "A",
				"b": 1,
				"c": true,
				/* slices */
				"d": []interface{}{"D1", "D2", "D3"},
				/* nested */
				"e": map[string]interface{}{
					"f": "F",
					"g": map[string]interface{}{
						"h": "H",
						"i": 1,
					},
					"j": 1,
				},
				/* nested slices */
				"k": []interface{}{
					map[string]interface{}{
						"l1": "L11",
						"m1": map[string]interface{}{
							"n1": "N11",
							"o1": 11,
						},
					},
					map[string]interface{}{
						"l2": "L21",
						"m2": map[string]interface{}{
							"n2": "N21",
							"o2": 12,
						},
					},
				},
			}
			src := Values{
				/* scalar */
				"b": 2,
				"c": nil,
				"x": "X",
				/* slices */
				"d": []interface{}{"D4"},
				"y": []interface{}{"X1", "X2"},
				/* nested */
				"e": map[string]interface{}{
					"g": map[string]interface{}{
						"i": 2,
						"w": "W",
					},
					"j": 2,
					"z": "Z",
				},
				/* nested slices */
				"k": []interface{}{
					map[string]interface{}{
						"l1": "L12",
					},
					map[string]interface{}{
						"m2": map[string]interface{}{
							"o2": 22,
						},
					},
					map[string]interface{}{
						"l3": "L32",
						"m3": map[string]interface{}{
							"n3": "N32",
							"o3": 32,
						},
					},
				},
			}
			exp := Values{
				/* scalar */
				"a": "A",
				"b": 2,
				"c": nil,
				"x": "X",
				/* slices */
				"d": []interface{}{"D4"},
				"y": []interface{}{"X1", "X2"},
				/* nested */
				"e": map[string]interface{}{
					"f": "F",
					"g": map[string]interface{}{
						"h": "H",
						"i": 2,
						"w": "W",
					},
					"j": 2,
					"z": "Z",
				},
				/* nested slices */
				"k": []interface{}{
					map[string]interface{}{
						"l1": "L12",
					},
					map[string]interface{}{
						"m2": map[string]interface{}{
							"o2": 22,
						},
					},
					map[string]interface{}{
						"l3": "L32",
						"m3": map[string]interface{}{
							"n3": "N32",
							"o3": 32,
						},
					},
				},
			}

			Expect(dst.Merge(src)).To(Succeed())
			Expect(dst).To(Equal(exp))
		})
	})

	Describe("Coalesce", func() {
		It("can deep merge another Values", func() {
			dst := Values{
				/* scalar */
				"a": "A",
				"b": 1,
				"c": nil,
				/* nested */
				"e": map[string]interface{}{
					"f": "F",
					"g": map[string]interface{}{
						"h": "H",
						"i": 1,
					},
					"j": 1,
				},
			}
			src := Values{
				/* scalar */
				"b": 2,
				"c": true,
				"x": "X",
				/* nested */
				"e": map[string]interface{}{
					"g": map[string]interface{}{
						"h": "H",
						"i": 1,
						"w": "W",
					},
					"j": 2,
					"z": "Z",
				},
			}
			exp := Values{
				/* scalar */
				"a": "A",
				"b": 1,
				"x": "X",
				/* nested */
				"e": map[string]interface{}{
					"f": "F",
					"g": map[string]interface{}{
						"h": "H",
						"i": 1,
						"w": "W",
					},
					"j": 1,
					"z": "Z",
				},
			}

			dst.Coalesce(src)
			Expect(dst).To(Equal(exp))
		})
	})

	Describe("AddHelmValue", func() {
		It("supports Helm style key and value format", func() {
			v := Values{
				"a": "A",
				"b": map[string]interface{}{
					"c": "C",
				},
				"d": []interface{}{"D1", "D2", "D3"},
				"e": []interface{}{
					map[string]interface{}{
						"f1": "F1",
						"g1": "G1",
					},
				},
			}

			Expect(v.AddHelmValue("x", "X")).To(Succeed())
			Expect(v.AddHelmValue("i.j.k", "K")).To(Succeed())
			Expect(v.AddHelmValue("i.j.l", "{L1,L2,L3}")).To(Succeed())
			Expect(v.AddHelmValue("b.c", "C2")).To(Succeed())
			Expect(v.AddHelmValue("d[0]", "D12")).To(Succeed())
			Expect(v.AddHelmValue("e[0].f1", "F12")).To(Succeed())
			Expect(v.AddHelmValue("e[1].h2", "H2")).To(Succeed())

			Expect(v).To(Equal(Values{
				"a": "A",
				"x": "X",
				"b": map[string]interface{}{
					"c": "C2",
				},
				"d": []interface{}{"D12", "D2", "D3"},
				"e": []interface{}{
					map[string]interface{}{
						"f1": "F12",
						"g1": "G1",
					},
					map[string]interface{}{
						"h2": "H2",
					},
				},
				"i": map[string]interface{}{
					"j": map[string]interface{}{
						"k": "K",
						"l": []interface{}{"L1", "L2", "L3"},
					},
				},
			}))
		})
	})

	Describe("AddFromYAML", func() {
		It("merges the values read from YAML", func() {
			v := Values{
				"a": "A",
				"b": map[string]interface{}{
					"c": map[string]interface{}{
						"d": "D",
					},
				},
				"y": "Y",
				"z": []interface{}{"Z1"},
			}
			y := `
a: A2
x: X1
b:
  x: X2
  c:
    d: D2
e:
- f1: F1
  g1: G1
- f2: F2
  g2: G2
`

			Expect(v.AddFromYAML(y)).To(Succeed())

			Expect(v).To(Equal(Values{
				"a": "A2",
				"x": "X1",
				"b": map[string]interface{}{
					"x": "X2",
					"c": map[string]interface{}{
						"d": "D2",
					},
				},
				"e": []interface{}{
					map[string]interface{}{
						"f1": "F1",
						"g1": "G1",
					},
					map[string]interface{}{
						"f2": "F2",
						"g2": "G2",
					},
				},
				"y": "Y",
				"z": []interface{}{"Z1"},
			}))
		})

		It("returns error when YAML is not parsable", func() {
			v := Values{}
			Expect(v.AddFromYAML("<xml></xml>")).NotTo(Succeed())
		})
	})
})
