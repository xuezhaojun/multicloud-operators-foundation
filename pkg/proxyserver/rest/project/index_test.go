package project

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestIndexClusterPermissionBySubject(t *testing.T) {
	tests := []struct {
		name        string
		input       runtime.Object
		expected    []string
		expectError bool
	}{
		{
			name: "valid object with clusterRoleBinding",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{
						"clusterRoleBinding": map[string]interface{}{
							"subject": map[string]interface{}{
								"kind": "User",
								"name": "test-user",
							},
						},
					},
				},
			},
			expected:    []string{"test-ns/test-name/User/test-user"},
			expectError: false,
		},
		{
			name: "valid object with roleBindings",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{
						"roleBindings": []interface{}{
							map[string]interface{}{
								"subject": map[string]interface{}{
									"kind": "Group",
									"name": "test-group",
								},
							},
						},
					},
				},
			},
			expected:    []string{"test-ns/test-name/Group/test-group"},
			expectError: false,
		},
		{
			name: "object with both clusterRoleBinding and roleBindings",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{
						"clusterRoleBinding": map[string]interface{}{
							"subject": map[string]interface{}{
								"kind": "User",
								"name": "user1",
							},
						},
						"roleBindings": []interface{}{
							map[string]interface{}{
								"subject": map[string]interface{}{
									"kind": "Group",
									"name": "group1",
								},
							},
							map[string]interface{}{
								"subject": map[string]interface{}{
									"kind": "User",
									"name": "user2",
								},
							},
						},
					},
				},
			},
			expected: []string{
				"test-ns/test-name/User/user1",
				"test-ns/test-name/Group/group1",
				"test-ns/test-name/User/user2",
			},
			expectError: false,
		},
		{
			name: "object with invalid subject",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{
						"roleBindings": []interface{}{
							map[string]interface{}{
								"subject": map[string]interface{}{}, // missing required fields
							},
						},
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "object with invalid roleBindings type",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{
						"roleBindings": "not-a-slice", // wrong type
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "object with no bindings",
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "test-ns",
						"name":      "test-name",
					},
					"spec": map[string]interface{}{}, // no bindings
				},
			},
			expected:    []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IndexClusterPermissionBySubject(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !sets.NewString(result...).Equal(sets.NewString(tt.expected...)) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestToSubject(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expected    *rbacv1.Subject
		expectError bool
	}{
		{
			name: "valid subject",
			input: map[string]interface{}{
				"subject": map[string]interface{}{
					"kind": "User",
					"name": "test-user",
				},
			},
			expected: &rbacv1.Subject{
				Kind: "User",
				Name: "test-user",
			},
			expectError: false,
		},
		{
			name: "missing subject",
			input: map[string]interface{}{
				"not-subject": "value",
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "invalid subject type",
			input: map[string]interface{}{
				"subject": "not-a-map",
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toSubject(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Kind != tt.expected.Kind || result.Name != tt.expected.Name {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
