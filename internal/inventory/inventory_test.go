// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inventory

import (
	"os"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/cli-utils/pkg/object"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func Test_Inventory(t *testing.T) {
	g := NewWithT(t)

	set1, err := readManifest("testdata/inventory1.yaml")
	if err != nil {
		t.Fatal(err)
	}

	inv1 := New()
	err = AddChangeSet(inv1, set1)
	g.Expect(err).ToNot(HaveOccurred())

	set2, err := readManifest("testdata/inventory2.yaml")
	if err != nil {
		t.Fatal(err)
	}

	inv2 := New()
	err = AddChangeSet(inv2, set2)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("lists objects in inventory", func(t *testing.T) {
		unList, err := List(inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(unList)).To(BeIdenticalTo(len(inv1.Entries)))

		mList, err := ListMetadata(inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(mList)).To(BeIdenticalTo(len(inv1.Entries)))
	})

	t.Run("diff objects in inventory", func(t *testing.T) {
		unList, err := Diff(inv2, inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(unList)).To(BeIdenticalTo(1))
		g.Expect(unList[0].GetName()).To(BeIdenticalTo("test2"))
	})
}

func Test_Merge(t *testing.T) {
	base := &fluxcdv1.ResourceInventory{
		Entries: []fluxcdv1.ResourceRef{
			{ID: "default_test2__ConfigMap", Version: "v1"},
			{ID: "default_test1__ConfigMap", Version: "v1"},
		},
	}
	overlay := &fluxcdv1.ResourceInventory{
		Entries: []fluxcdv1.ResourceRef{
			{ID: "default_test3_apps_Deployment", Version: "v1"},
			{ID: "default_test1__ConfigMap", Version: "v2"},
		},
	}

	t.Run("dedups entries with overlay version precedence sorted by ID", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(base, overlay)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v2"},
			{ID: "default_test2__ConfigMap", Version: "v1"},
			{ID: "default_test3_apps_Deployment", Version: "v1"},
		}))
	})

	t.Run("treats nil base as empty", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(nil, overlay)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v2"},
			{ID: "default_test3_apps_Deployment", Version: "v1"},
		}))
	})

	t.Run("treats nil overlay as empty", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(base, nil)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v1"},
			{ID: "default_test2__ConfigMap", Version: "v1"},
		}))
	})

	t.Run("returns empty inventory for nil inputs", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(nil, nil)
		g.Expect(result.Entries).To(BeEmpty())
	})
}

func readManifest(manifest string) (*ssa.ChangeSet, error) {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}

	objects, err := ssautil.ReadObjects(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	cs := ssa.NewChangeSet()

	for _, o := range objects {
		cse := ssa.ChangeSetEntry{
			ObjMetadata:  object.UnstructuredToObjMetadata(o),
			GroupVersion: o.GroupVersionKind().Version,
			Subject:      ssautil.FmtUnstructured(o),
			Action:       ssa.CreatedAction,
		}
		cs.Add(cse)
	}

	return cs, nil
}

func Test_AddChangeSetForStep(t *testing.T) {
	g := NewWithT(t)

	cs := ssa.NewChangeSet()
	cs.Add(ssa.ChangeSetEntry{
		ObjMetadata: object.ObjMetadata{
			Namespace: "default",
			Name:      "cm1",
			GroupKind: schema.GroupKind{Kind: "ConfigMap"},
		},
		GroupVersion: "v1",
		Action:       ssa.CreatedAction,
	})
	cs.Add(ssa.ChangeSetEntry{
		ObjMetadata: object.ObjMetadata{
			Namespace: "default",
			Name:      "cm2",
			GroupKind: schema.GroupKind{Kind: "ConfigMap"},
		},
		GroupVersion: "v1",
		Action:       ssa.CreatedAction,
	})

	t.Run("tags entries with step index", func(t *testing.T) {
		inv := New()
		idx := 1
		err := AddChangeSetForStep(inv, cs, &idx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inv.Entries).To(HaveLen(2))
		for _, entry := range inv.Entries {
			g.Expect(*entry.StepIndex).To(Equal(1))
		}
	})

	t.Run("nil step index leaves step unset", func(t *testing.T) {
		inv := New()
		err := AddChangeSetForStep(inv, cs, nil)
		g.Expect(err).ToNot(HaveOccurred())
		for _, entry := range inv.Entries {
			g.Expect(entry.StepIndex).To(BeNil())
		}
	})

	t.Run("backward compatible with AddChangeSet", func(t *testing.T) {
		inv := New()
		err := AddChangeSet(inv, cs)
		g.Expect(err).ToNot(HaveOccurred())
		for _, entry := range inv.Entries {
			g.Expect(entry.Step).To(BeEmpty())
		}
	})
}

func Test_MergeWithSteps(t *testing.T) {
	idx1 := 1
	idx2 := 2

	t.Run("overlay step takes precedence", func(t *testing.T) {
		g := NewWithT(t)

		base := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx1},
			},
		}
		overlay := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx2},
			},
		}

		result := Merge(base, overlay)
		g.Expect(result.Entries).To(HaveLen(1))
		g.Expect(*result.Entries[0].StepIndex).To(Equal(2))
	})

	t.Run("preserves steps from both inventories", func(t *testing.T) {
		g := NewWithT(t)

		base := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx1},
			},
		}
		overlay := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm2__ConfigMap", Version: "v1", StepIndex: &idx2},
			},
		}

		result := Merge(base, overlay)
		g.Expect(result.Entries).To(HaveLen(2))
		g.Expect(*result.Entries[0].StepIndex).To(Equal(1))
		g.Expect(*result.Entries[1].StepIndex).To(Equal(2))
	})
}

func Test_ListByStepsReversed(t *testing.T) {
	idx0 := 0
	idx1 := 1
	idx2 := 2

	t.Run("returns groups in reverse step order", func(t *testing.T) {
		g := NewWithT(t)

		inv := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx0},
				{ID: "default_cm2__ConfigMap", Version: "v1", StepIndex: &idx1},
				{ID: "default_cm3__ConfigMap", Version: "v1", StepIndex: &idx2},
			},
		}

		result, err := ListByStepsReversed(inv)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(HaveLen(3))
		// Reverse order: idx2 first, idx0 last.
		g.Expect(result[0][0].GetName()).To(Equal("cm3"))
		g.Expect(result[1][0].GetName()).To(Equal("cm2"))
		g.Expect(result[2][0].GetName()).To(Equal("cm1"))
	})

	t.Run("handles multiple objects per step", func(t *testing.T) {
		g := NewWithT(t)

		inv := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx0},
				{ID: "default_cm2__ConfigMap", Version: "v1", StepIndex: &idx1},
				{ID: "default_cm3__ConfigMap", Version: "v1", StepIndex: &idx1},
				{ID: "default_cm4__ConfigMap", Version: "v1", StepIndex: &idx2},
			},
		}

		result, err := ListByStepsReversed(inv)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(HaveLen(3))
		// idx2 first, then idx1 (2 objects), then idx0.
		g.Expect(result[0]).To(HaveLen(1))
		g.Expect(result[0][0].GetName()).To(Equal("cm4"))

		g.Expect(result[1]).To(HaveLen(2))
		g.Expect(result[1][0].GetName()).To(Equal("cm2"))
		g.Expect(result[1][1].GetName()).To(Equal("cm3"))

		g.Expect(result[2]).To(HaveLen(1))
		g.Expect(result[2][0].GetName()).To(Equal("cm1"))
	})

	t.Run("nil step index entries are deleted first", func(t *testing.T) {
		g := NewWithT(t)

		inv := &fluxcdv1.ResourceInventory{
			Entries: []fluxcdv1.ResourceRef{
				{ID: "default_cm1__ConfigMap", Version: "v1", StepIndex: &idx0},
				{ID: "default_cm2__ConfigMap", Version: "v1", StepIndex: nil},
			},
		}

		result, err := ListByStepsReversed(inv)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(HaveLen(2))
		// Nil index first, then idx0.
		g.Expect(result[0][0].GetName()).To(Equal("cm2"))
		g.Expect(result[1][0].GetName()).To(Equal("cm1"))
	})

	t.Run("returns nil for empty inventory", func(t *testing.T) {
		g := NewWithT(t)

		result, err := ListByStepsReversed(&fluxcdv1.ResourceInventory{})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(BeNil())
	})

	t.Run("returns nil for nil inventory", func(t *testing.T) {
		g := NewWithT(t)

		result, err := ListByStepsReversed(nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(BeNil())
	})
}



