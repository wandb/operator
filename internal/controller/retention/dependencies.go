package retention

import (
	"fmt"
	"maps"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
)

// ValidatePolicies is shared by admission and reconciliation. Named object
// stores use precisely the same default fallback as the provisioning code.
func ValidatePolicies(w *apiv2.WeightsAndBiases) error {
	if _, err := Policy(w, apiv2.ManagedInfraSpec{}); err != nil {
		return err
	}
	if _, err := Inventory(w); err != nil {
		return err
	}
	check := func(component, key, preferred string, infra apiv2.ManagedInfraSpec) error {
		policy, err := Policy(w, infra)
		if err != nil {
			return err
		}
		if policy != apiv2.DetachOnDelete {
			return nil
		}
		store, resolved := w.Spec.ObjectStore[preferred]
		storeKey := preferred
		if !resolved {
			store, resolved = w.Spec.ObjectStore[apiv2.DefaultInstanceName]
			storeKey = apiv2.DefaultInstanceName
		}
		if !resolved {
			return nil
		}
		dependency := apiv2.ManagedInfraSpec{}
		if store.ManagedObjectStore != nil {
			dependency = store.ManagedObjectStore.ManagedInfraSpec
		} else if component == "kafka" {
			return nil
		}
		// ClickHouse references the source Secret for static credentials. Treat
		// secret-backed/unknown auth as a dependency without reading secret values.
		if store.ExternalObjectStore != nil && store.ExternalObjectStore.AccessKey.Value == "" && store.ExternalObjectStore.AccessKey.SecretKeyRef() == nil {
			return nil
		}
		p, err := Policy(w, dependency)
		if err != nil {
			return err
		}
		if p == apiv2.PurgeOnDelete {
			return fmt.Errorf("spec.%s[%s] has detach but requires spec.objectStore[%s], whose retentionPolicy.onDelete is purge; retain the dependency as well", component, key, storeKey)
		}
		return nil
	}
	for _, key := range slices.Sorted(maps.Keys(w.Spec.ClickHouse)) {
		spec := w.Spec.ClickHouse[key]
		if s := spec.ManagedClickHouse; s != nil {
			if err := check("clickhouse", key, "clickhouse", s.ManagedInfraSpec); err != nil {
				return err
			}
		}
	}
	if s := w.Spec.Kafka.ManagedKafka; s != nil {
		return check("kafka", apiv2.DefaultInstanceName, "bufstream", s.ManagedInfraSpec)
	}
	return nil
}

func (s *Session) validateDependencies() error {
	if err := ValidatePolicies(s.parent); err != nil {
		return err
	}
	// Recorded descendants can share an owner even after spec entries disappear.
	for _, r := range s.Ledger.Resources {
		if r.Policy != apiv2.DetachOnDelete && r.Policy != apiv2.PurgeOnDelete {
			return fmt.Errorf("invalid stored retention policy for %s", r.Key())
		}
	}
	policies := map[string]apiv2.OnDeletePolicy{}
	for _, r := range s.Ledger.Resources {
		policies[InstanceID(r.Component, r.Instance)] = r.Policy
	}
	for consumer, dependencies := range s.Ledger.Dependencies {
		if policies[consumer] != apiv2.DetachOnDelete {
			continue
		}
		for _, dependency := range dependencies {
			if policies[dependency] == apiv2.PurgeOnDelete {
				return fmt.Errorf("retained instance %s depends on purged instance %s", consumer, dependency)
			}
		}
	}
	return nil
}

func Dependencies(w *apiv2.WeightsAndBiases) map[string][]string {
	result := map[string][]string{}
	add := func(component, key, preferred string) {
		store, ok := w.Spec.ObjectStore[preferred]
		selected := preferred
		if !ok {
			store, ok = w.Spec.ObjectStore[apiv2.DefaultInstanceName]
			selected = apiv2.DefaultInstanceName
		}
		if !ok {
			return
		}
		if store.ManagedObjectStore == nil {
			if component == "kafka" || store.ExternalObjectStore == nil {
				return
			}
			if store.ExternalObjectStore.AccessKey.Value == "" && store.ExternalObjectStore.AccessKey.SecretKeyRef() == nil {
				return
			}
		}
		result[InstanceID(component, key)] = []string{InstanceID("objectstore", selected)}
	}
	for key, spec := range w.Spec.ClickHouse {
		if spec.ManagedClickHouse != nil {
			add("clickhouse", key, "clickhouse")
		}
	}
	if w.Spec.Kafka.ManagedKafka != nil {
		add("kafka", apiv2.DefaultInstanceName, "bufstream")
	}
	return result
}
