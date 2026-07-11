package biz

var serviceAnnotations = map[string]map[string]interface{}{}

// GetApiAnnotation gets the annotation of certain service method in IDL.
func GetApiAnnotation(apiPattern string) (map[string]interface{}, bool) {
	m, ok := serviceAnnotations[apiPattern]
	return m, ok
}

// GetServiceAnnotations gets the annotations of all methods in IDL.
func GetServiceAnnotations() map[string]map[string]interface{} {
	return serviceAnnotations
}
