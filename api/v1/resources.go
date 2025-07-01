package v1

// ClusterTemplate represents a ClusterClass template available for cluster creation.
type ClusterTemplate struct {
	Name               string             `json:"name"`
	Namespace          string             `json:"namespace"`
	Description        string             `json:"description"`
	Provider           string             `json:"provider"`
	KubernetesVersions []string           `json:"kubernetes_versions"`
	Variables          []TemplateVariable `json:"variables"`
	Labels             map[string]string  `json:"labels"`
	Annotations        map[string]string  `json:"annotations"`
}

// TemplateVariable describes a variable that can be set when creating a cluster from a template.
type TemplateVariable struct {
	Name        string      `json:"name"`
	Required    bool        `json:"required"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description"`
	Example     interface{} `json:"example,omitempty"`
}
