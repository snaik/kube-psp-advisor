package types

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/api/policy/v1beta1"
	v1rbac "k8s.io/api/rbac/v1"
)

const (
	rbacV1APIVersion = "rbac.authorization.k8s.io/v1"
	rbacAPIGroup     = "rbac.authorization.k8s.io"
	Role             = "Role"
	RoleBinding      = "RoleBinding"
	ServiceAccount   = "ServiceAccount"
)

type SASecuritySpecList []*SASecuritySpec

func (sl SASecuritySpecList) Less(i, j int) bool {
	keyI := fmt.Sprintf("%s:%s", sl[i].Namespace, sl[i].ServiceAccount)
	keyJ := fmt.Sprintf("%s:%s", sl[j].Namespace, sl[j].ServiceAccount)

	return keyI < keyJ
}

func (sl SASecuritySpecList) Len() int { return len(sl) }

func (sl SASecuritySpecList) Swap(i, j int) { sl[i], sl[j] = sl[j], sl[i] }

type SASecuritySpec struct {
	PSPName string // psp name

	ServiceAccount string // serviceAccount

	Namespace string // namespace

	ContainerSecuritySpecList []ContainerSecuritySpec

	PodSecuritySpecList []PodSecuritySpec
}

func NewSASecuritySpec(ns, sa string) *SASecuritySpec {
	return &SASecuritySpec{
		ServiceAccount:            sa,
		Namespace:                 ns,
		ContainerSecuritySpecList: []ContainerSecuritySpec{},
		PodSecuritySpecList:       []PodSecuritySpec{},
	}
}

// IsDefaultServiceAccount returns whether the service account is default
func (s *SASecuritySpec) IsDefaultServiceAccount() bool {
	return s.ServiceAccount == "default"
}

// AddContainerSecuritySpec adds container security spec object to the associated service account
func (s *SASecuritySpec) AddContainerSecuritySpec(css ContainerSecuritySpec) {
	s.ContainerSecuritySpecList = append(s.ContainerSecuritySpecList, css)
}

// AddPodSecuritySpec adds pod security spec object to the associated service account
func (s *SASecuritySpec) AddPodSecuritySpec(pss PodSecuritySpec) {
	s.PodSecuritySpecList = append(s.PodSecuritySpecList, pss)
}

// GeneratePSPName generates psp name
func (s *SASecuritySpec) GeneratePSPName() string {
	if s.PSPName == "" {
		s.PSPName = fmt.Sprintf("psp-for-%s-%s", s.Namespace, s.ServiceAccount)
	}

	return s.PSPName
}

// GenerateComment generate comments for the psp grants (no psp will be created for default service account)
func (s *SASecuritySpec) GenerateComment() string {
	decision := "will be"

	if s.IsDefaultServiceAccount() {
		decision = "will NOT be"
	}

	commentsForWorkloads := []string{}
	comment := fmt.Sprintf("# Pod security policies %s created for service account '%s' in namespace '%s' with following workdloads:\n", decision, s.ServiceAccount, s.Namespace)
	for _, wlImg := range s.GetWorkloadImages() {
		commentsForWorkloads = append(commentsForWorkloads, fmt.Sprintf("#\t%s", wlImg))
	}

	comment += strings.Join(commentsForWorkloads, "\n")
	return comment
}

// GetWorkloadImages returns a list of workload images in the format of "kind, Name, Image Name"
func (s *SASecuritySpec) GetWorkloadImages() []string {
	workLoadImageList := []string{}

	for _, css := range s.ContainerSecuritySpecList {
		workLoadImage := fmt.Sprintf("Kind: %s, Name: %s, Image: %s", css.Metadata.Kind, css.Metadata.Name, css.ImageName)
		workLoadImageList = append(workLoadImageList, workLoadImage)
	}

	return workLoadImageList
}

// GenerateRole creates a role object contains the privilege to use the psp
func (s *SASecuritySpec) GenerateRole() *v1rbac.Role {
	roleName := fmt.Sprintf("use-psp-by-%s:%s", s.Namespace, s.ServiceAccount)

	rule := v1rbac.PolicyRule{
		Verbs:         []string{"use"},
		APIGroups:     []string{"policy"},
		Resources:     []string{"podsecuritypolicies"},
		ResourceNames: []string{s.GeneratePSPName()},
	}

	return &v1rbac.Role{
		TypeMeta: v1.TypeMeta{
			Kind:       Role,
			APIVersion: rbacV1APIVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      roleName,
		},
		Rules: []v1rbac.PolicyRule{rule},
	}
}

// GenerateRoleBinding creates a rolebinding for the service account to use the psp
func (s *SASecuritySpec) GenerateRoleBinding() *v1rbac.RoleBinding {
	roleBindingName := fmt.Sprintf("use-psp-by-%s:%s-binding", s.Namespace, s.ServiceAccount)
	roleName := fmt.Sprintf("use-psp-by-%s:%s", s.Namespace, s.ServiceAccount)

	return &v1rbac.RoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       RoleBinding,
			APIVersion: rbacV1APIVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      roleBindingName,
		},
		Subjects: []v1rbac.Subject{
			{Kind: ServiceAccount, Name: s.ServiceAccount, Namespace: s.Namespace},
		},
		RoleRef: v1rbac.RoleRef{
			APIGroup: rbacAPIGroup,
			Kind:     Role,
			Name:     roleName,
		},
	}
}

type PSPGrant struct {
	Comment           string
	PodSecurityPolicy *v1beta1.PodSecurityPolicy
	Role              *v1rbac.Role
	RoleBinding       *v1rbac.RoleBinding
}