package apibinding

import (
	o "github.com/onsi/gomega"

	exutil "github.com/kcp-dev/kcp-tests/test/extended/util"
)

// APIBinding struct defination
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

// ABOption use function option mode to change the default values of Apibinding attributes
type ABOption func(*APIBinding)

// SetAPIBindingName replace the default value of APIBinding name
func SetAPIBindingName(name string) ABOption {
	return func(a *APIBinding) {
		a.Metadata.Name = name
	}
}

// SetAPIBindingReferencePath replace the default value of APIBinding workspace reference path
func SetAPIBindingReferencePath(path string) ABOption {
	return func(a *APIBinding) {
		a.Spec.Reference.Workspace.Path = path
	}
}

// SetAPIBindingReferenceExportName replace the default value of APIBinding workspace reference exportName
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

// Create APIBinding CR
func (apb *APIBinding) Create(k *exutil.CLI) {
	outputFile, err := exutil.StructMarshalOutputToFile(apb, apb.Metadata.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = k.ApplyResourceFromSpecificFile(outputFile)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete APIBinding CR
func (apb *APIBinding) Delete(k *exutil.CLI) {
	err := k.WithoutNamespace().WithoutKubeconf().Run("delete").Args("apibinding", apb.Metadata.Name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}
