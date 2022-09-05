package apibinding

import (
	"fmt"

	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

// APIBinding struct definition
type APIBinding struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Reference struct {
			Workspace struct {
				Path       string `json:"path"`
				ExportName string `json:"exportName"`
			} `json:"workspace"`
		} `json:"reference"`
	} `json:"spec"`
}

// ABOption uses function option mode to change the default values of APIBinding attributes
type ABOption func(*APIBinding)

// SetAPIBindingName sets the APIBinding's name
func SetAPIBindingName(name string) ABOption {
	return func(a *APIBinding) {
		a.Metadata.Name = name
	}
}

// SetAPIBindingReferencePath sets the APIBinding's workspace reference path
func SetAPIBindingReferencePath(path string) ABOption {
	return func(a *APIBinding) {
		a.Spec.Reference.Workspace.Path = path
	}
}

// SetAPIBindingReferenceExportName sets the APIBinding's workspace reference exportName
func SetAPIBindingReferenceExportName(exportName string) ABOption {
	return func(a *APIBinding) {
		a.Spec.Reference.Workspace.ExportName = exportName
	}
}

// NewAPIBinding create a new customized APIBinding
func NewAPIBinding(opts ...ABOption) APIBinding {
	defaultAPIBinding := APIBinding{
		APIVersion: "apis.kcp.dev/v1alpha1",
		Kind:       "APIBinding",
		Metadata: struct {
			Name string `json:"name"`
		}{Name: "e2e-apibinding-" + exutil.GetRandomString()},
		Spec: struct {
			Reference struct {
				Workspace struct {
					Path       string `json:"path"`
					ExportName string `json:"exportName"`
				} `json:"workspace"`
			} `json:"reference"`
		}{},
	}
	for _, o := range opts {
		o(&defaultAPIBinding)
	}
	return defaultAPIBinding
}

// Create APIBinding
func (apb *APIBinding) Create(k *exutil.CLI) {
	apb.CreateAsExpectedResult(k, true, "created")
}

// CreateAsExpectedResult creates APIBinding CR and checks the created result is as expected
func (apb *APIBinding) CreateAsExpectedResult(k *exutil.CLI, successFlag bool, containsMsg string) {
	outputFile, err := exutil.StructMarshalOutputToFile(apb, apb.Metadata.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	msg, applyErr := k.ApplyResourceFromSpecificFile(outputFile)
	if successFlag {
		o.Expect(applyErr).ShouldNot(o.HaveOccurred())
		o.Expect(msg).Should(o.ContainSubstring(containsMsg))
	} else {
		o.Expect(applyErr).Should(o.HaveOccurred())
		o.Expect(fmt.Sprint(msg)).Should(o.ContainSubstring(containsMsg))
	}
}

// Delete APIBinding
func (apb *APIBinding) Delete(k *exutil.CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("apibinding", apb.Metadata.Name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Clean the APIBinding resource
func (apb *APIBinding) Clean(k *exutil.CLI) {
	k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("apibinding", apb.Metadata.Name).Execute()
}
