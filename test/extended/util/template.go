package util

// ApplyResourceFromTemplate apply the changes to the cluster resource.
// For ex: ApplyResourceFromTemplate(c, "TEMPLATE_FILE")
func ApplyResourceFromTemplate(c *CLI, filepath string) (string, error) {
	return resourceFromTemplate(c, false, "", filepath, map[string]string{})
}

// ApplyResourceFromTemplateWithVariables apply the changes to the cluster resource with args.
// For ex: ApplyResourceFromTemplateWithVariables(c, "TEMPLATE_FILE", map{"VAR_1":"VALUE_1"})
func ApplyResourceFromTemplateWithVariables(c *CLI, filepath string, args map[string]string) (string, error) {
	return resourceFromTemplate(c, false, "", filepath, args)
}

// ApplyNsResourceFromTemplate apply changes to the ns resource.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: ApplyNsResourceFromTemplate(c, "NAMESPACE", "TEMPLATE_FILE")
func ApplyNsResourceFromTemplate(c *CLI, namespace string, filepath string) (string, error) {
	return resourceFromTemplate(c, false, namespace, filepath, map[string]string{})
}

// ApplyNsResourceFromTemplateWithVariables apply changes to the ns resource.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: ApplyNsResourceFromTemplateWithVariables(c, "NAMESPACE", "TEMPLATE_FILE", map{"VAR_1":"VALUE_1"})
func ApplyNsResourceFromTemplateWithVariables(c *CLI, namespace string, filepath string, variables map[string]string) (string, error) {
	return resourceFromTemplate(c, false, namespace, filepath, variables)
}

// CreateResourceFromTemplate create resource from the template.
// For ex: CreateResourceFromTemplate(c, "TEMPLATE_FILE")
func CreateResourceFromTemplate(c *CLI, filepath string) (string, error) {
	return resourceFromTemplate(c, true, "", filepath, map[string]string{})
}

// CreateResourceFromTemplateWithVariables create resource from the template.
// For ex: CreateResourceFromTemplateWithVariables(c, "TEMPLATE_FILE", map{"VAR_1":"VALUE_1"})
func CreateResourceFromTemplateWithVariables(c *CLI, filepath string, variables map[string]string) (string, error) {
	return resourceFromTemplate(c, true, "", filepath, variables)
}

// CreateNsResourceFromTemplate create ns resource from the template.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: CreateNsResourceFromTemplate(c, "NAMESPACE", "TEMPLATE_FILE")
func CreateNsResourceFromTemplate(c *CLI, namespace string, filepath string) (string, error) {
	return resourceFromTemplate(c, true, namespace, filepath, map[string]string{})
}

// CreateNsResourceFromTemplateWithVariables create ns resource from the template.
// No need to add a namespace parameter in the template file as it can be provided as a function argument.
// For ex: CreateNsResourceFromTemplateWithVariables(c, "NAMESPACE", "TEMPLATE_FILE", map{"VAR_1":"VALUE_1"})
func CreateNsResourceFromTemplateWithVariables(c *CLI, namespace string, filepath string, variables map[string]string) (string, error) {
	return resourceFromTemplate(c, true, namespace, filepath, variables)
}

func resourceFromTemplate(c *CLI, create bool, namespace string, filepath string, variables map[string]string) (string, error) {
	if len(variables) > 0 {
		filepath = ParseFileVariables(filepath, variables)
	}
	parameters := []string{"-f", filepath}
	if namespace != "" {
		parameters = append(parameters, "-n", namespace)
	}

	var (
		err    error
		output string
	)
	if create {
		output, err = c.Run("create").Args(parameters...).Output()
	} else {
		output, err = c.Run("apply").Args(parameters...).Output()
	}
	return output, err
}
