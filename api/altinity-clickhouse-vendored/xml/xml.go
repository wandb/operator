// Copyright 2019 Altinity Ltd and/or its affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xml

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/wandb/operator/api/altinity-clickhouse-vendored/util"
)

type xmlNode struct {
	children []*xmlNode
	tag      string
	value    setting
}

const (
	eol   = "\n"
	noEol = ""
)

type setting interface {
	fmt.Stringer
	IsEmpty() bool
	IsScalar() bool
	IsVector() bool
	Attributes() string
	VectorOfStrings() []string
	IsEmbed() bool
}

type settings interface {
	Len() int
	WalkNames(func(name string))
	GetA(string) any
}

// GenerateFromSettings creates XML representation from the provided settings
func GenerateFromSettings(w io.Writer, settings settings, prefix string) {
	if settings.Len() == 0 {
		return
	}

	// `paths` is a sorted set of normalized paths (maps keys) from settings
	paths := make([]string, 0, settings.Len())

	// `data` is a copy of settings with:
	// 1. paths (map keys) are normalized in terms of trimmed '/'
	// 2. all map keys listed in 'excludes' are excluded
	data := make(map[string]string)
	// Skip excluded paths
	settings.WalkNames(func(name string) {
		// 'name' may be non-normalized, and may have starting or trailing '/'
		// 'path' is normalized path without starting and trailing '/', ex.: 'test/quotas'
		path := normalizePath(prefix, name)
		if path == "" {
			return
		}
		paths = append(paths, path)
		data[path] = name
	})
	sort.Strings(paths)

	// `xmlTreeRoot` - root of the XML tree data structure
	xmlTreeRoot := new(xmlNode)

	// Read all tags and values into the tree structure
	for _, path := range paths {
		// Split path (test/quotas) into tags which would be 'test' and 'quota'
		tags := strings.Split(path, "/")
		if len(tags) == 0 {
			// Empty path? Should not be, but double check
			continue
		}
		name := data[path]
		xmlTreeRoot.addBranch(tags, settings.GetA(name).(setting))
	}

	// build XML into writer
	xmlTreeRoot.buildXML(w, 0, 4)
}

// normalizePath makes 'prefix/a/b/c' out of 'prefix' + '/a//b///c////'
// Important - leading '/' is removed!
func normalizePath(prefix, path string) string {
	// Normalize '//' to '/'
	re := regexp.MustCompile("//+")
	path = re.ReplaceAllString(path, "/")
	// Cut all leading and trailing '/'
	path = strings.Trim(path, "/")
	if len(prefix) > 0 {
		return prefix + "/" + path
	}

	return path
}

// addBranch ensures branch exists and assign value to the last tagged node
func (n *xmlNode) addBranch(tags []string, setting setting) {
	node := n
	for _, tag := range tags {
		node = node.addChild(tag)
	}
	node.value = setting
}

// addChild add new or return existing child with matching tag
func (n *xmlNode) addChild(tag string) *xmlNode {
	if n.children == nil {
		n.children = make([]*xmlNode, 0, 0)
	}

	// Check for such tag exists
	for i := range n.children {
		if n.children[i].tag == tag {
			// Already have such a tag
			return n.children[i]
		}
	}

	// No this is new tag - add as a child
	node := &xmlNode{
		tag: tag,
	}
	n.children = append(n.children, node)

	return node
}

func (n *xmlNode) NoValue() bool {
	return (n.value == nil) || n.value.IsEmpty()
}

// buildXML generates XML from xmlNode type linked list
func (n *xmlNode) buildXML(w io.Writer, indent, tabSize uint8) {
	switch {
	case n.NoValue():
		// No value node, may have nested tags
		n.writeTagNoValue(w, "", indent, tabSize)
		return

	case n.value.IsScalar():
		// ScalarString node
		n.writeTagWithValue(w, n.value.String(), n.value.Attributes(), indent, n.value.IsEmbed())
		return

	case n.value.IsVector():
		// VectorOfStrings node
		for _, value := range n.value.VectorOfStrings() {
			n.writeTagWithValue(w, value, n.value.Attributes(), indent, n.value.IsEmbed())
		}
	}
}

// writeTagNoValue prints tag which has no value, But it may have nested tags
// <a>
//
//	<b>...</b>
//
// </a>
func (n *xmlNode) writeTagNoValue(w io.Writer, attributes string, indent, tabSize uint8) {
	n.writeTagOpen(w, indent, attributes, eol)
	for i := range n.children {
		n.children[i].buildXML(w, indent+tabSize, tabSize)
	}
	n.writeTagClose(w, indent, eol)
}

// writeTagWithValue prints tag with value. But it must have no children,
// and children are not printed
// <tag>value</tag>
// OR
// <tag>
// embedded value NB - printed w/o indent
// </tag>
func (n *xmlNode) writeTagWithValue(w io.Writer, value string, attributes string, indent uint8, embedded bool) {
	// TODO fix this properly
	// Used in tests
	if value == "_removed_" || value == "_remove_" {
		attributes = " remove=\"1\""
		value = ""
	}

	if embedded {
		// <tag>
		// embedded value NB - printed w/o indent
		// </tag>
		n.writeTagOpen(w, indent, attributes, eol)
		n.writeValue(w, value)
		n.writeTagClose(w, indent, eol)
	} else {
		// <tag>value</tag>
		n.writeTagOpen(w, indent, attributes, noEol)
		n.writeValue(w, value)
		n.writeTagClose(w, 0, eol)
	}
}

// writeTagOpen prints open XML tag into io.Writer
func (n *xmlNode) writeTagOpen(w io.Writer, indent uint8, attributes string, eol string) {
	n.writeTag(w, indent, attributes, true, eol)
}

// writeTagClose prints close XML tag into io.Writer
func (n *xmlNode) writeTagClose(w io.Writer, indent uint8, eol string) {
	n.writeTag(w, indent, "", false, eol)
}

// writeTag prints XML tag into io.Writer
func (n *xmlNode) writeTag(w io.Writer, indent uint8, attributes string, openTag bool, eol string) {
	if n.tag == "" {
		return
	}

	// We have to separate indent and no-indent cases, because event target pattern is like
	// "%0s</%s> - meaning we do not want to print leading spaces, having " " in Fprint inserts one space
	if indent > 0 {
		pattern := ""
		if openTag {
			// pattern would be: %4s<%s%s>%s
			pattern = fmt.Sprintf("%%%ds<%%s%%s>%%s", indent)
			util.Fprintf(w, pattern, " ", n.tag, attributes, eol)
		} else {
			// pattern would be: %4s</%s>%s
			pattern = fmt.Sprintf("%%%ds</%%s>%%s", indent)
			util.Fprintf(w, pattern, " ", n.tag, eol)
		}
	} else {
		if openTag {
			// pattern would be: <%s%s>%s
			util.Fprintf(w, "<%s%s>%s", n.tag, attributes, eol)
		} else {
			// pattern would be: </%s>%s
			util.Fprintf(w, "</%s>%s", n.tag, eol)
		}
	}
}

// writeValue prints XML value into io.Writer
func (n *xmlNode) writeValue(w io.Writer, value string) {
	util.Fprintf(w, "%s", value)
}
