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

package v1

// DistributedDDL defines distributedDDL section of .spec.defaults
type DistributedDDL struct {
	Profile string `json:"profile,omitempty" yaml:"profile"`
}

// NewDistributedDDL creates new DistributedDDL
func NewDistributedDDL() *DistributedDDL {
	return new(DistributedDDL)
}

// HasProfile checks whether profile is present
func (d *DistributedDDL) HasProfile() bool {
	if d == nil {
		return false
	}
	return len(d.Profile) > 0
}

// GetProfile gets profile
func (d *DistributedDDL) GetProfile() string {
	if d == nil {
		return ""
	}
	return d.Profile
}

// MergeFrom merges from specified source
func (d *DistributedDDL) MergeFrom(from *DistributedDDL, _type MergeType) *DistributedDDL {
	if from == nil {
		return d
	}

	if d == nil {
		d = NewDistributedDDL()
	}

	switch _type {
	case MergeTypeFillEmptyValues:
		if d.Profile == "" {
			d.Profile = from.Profile
		}
	case MergeTypeOverrideByNonEmptyValues:
		if from.Profile != "" {
			// Override by non-empty values only
			d.Profile = from.Profile
		}
	}

	return d
}
