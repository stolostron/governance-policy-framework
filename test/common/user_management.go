// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const ocpConfigNs = "openshift-config"

// k8sJSONPatch represents a Kubernetes patch of type JSON (i.e. types.JSONPatchType).
type k8sJSONPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// OCPUser represents an OpenShift user to be created on a cluster.
type OCPUser struct {
	// If a namespace is not provided, a cluster role binding is created instead of a role binding.
	ClusterRoles        []types.NamespacedName
	ClusterRoleBindings []string
	Password            string
	Username            string
}

// GenerateInsecurePassword is a random password generator from 15-30 bytes. It is insecure
// since the characters are limited to just hex values (i.e. 1-9,a-f) from the random bytes. An
// error is returned if the random bytes cannot be read.
func GenerateInsecurePassword() (string, error) {
	// A password ranging from 15-30 bytes
	pwSize := rand.Intn(15) + 15
	bytes := make([]byte, pwSize)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetKubeConfig will generate a kubeconfig file based on an OpenShift user. The path of the
// kubeconfig file is returned. It is the responsibility of the caller to delete the kubeconfig file
// after use.
func GetKubeConfig(server, username, password string) (string, error) {
	// Create a temporary file for the kubeconfig that the `oc login` command will generate
	f, err := os.CreateTemp("", "e2e-kubeconfig")
	if err != nil {
		return "", fmt.Errorf("failed to create the temporary kubeconfig")
	}
	kubeconfigPath := f.Name()
	// Close the file immediately so that the `oc login` command can use the file
	err = f.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close the temporary kubeconfig")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"oc",
		"--kubeconfig="+kubeconfigPath,
		"login",
		"--server="+server,
		"-u",
		username,
		"-p",
		password,
		"--insecure-skip-tls-verify=true",
	)
	// In some environments, `--kubeconfig` doesn't seem to be enough.
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(kubeconfigPath)
		return "", fmt.Errorf("failed to login with user '%s': %s", username, string(output))
	}

	return kubeconfigPath, nil
}

// CreateOCPUser will create an OpenShift user on a cluster, configure the identity provider for
// that user, and add the desired roles to the user. This function is idempotent.
func CreateOCPUser(
	client kubernetes.Interface, dynamicClient dynamic.Interface, secretName string, user OCPUser,
) error {
	// Hash the password in the format expected by an htpasswd file.
	passwordBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf(
			"failed to generate a bcrypt password hash for the user %s: %w", user.Username, err,
		)
	}

	// Create a secret to hold the generated htpasswd file with the user's credentials.
	htpasswd := []byte(fmt.Sprintf("%s:%s\n", user.Username, string(passwordBytes)))
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName},
		Data:       map[string][]byte{"htpasswd": htpasswd},
		Type:       corev1.SecretTypeOpaque,
	}
	_, err = client.CoreV1().Secrets(ocpConfigNs).Create(
		context.TODO(), &secret, metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create the secret %s/%s: %w", ocpConfigNs, secretName, err)
	}

	// Configure the identity provider with this new htpasswd.
	err = addHtPasswd(dynamicClient, secretName)
	if err != nil {
		return fmt.Errorf(
			"failed to configure the OpenShift identity provider for these users: %w", err,
		)
	}

	// Add the desired roles to the new user.
	err = addClusterRoleBindings(client, user)
	if err != nil {
		return err
	}

	return addClusterRoles(client, user)
}

// addHtPasswd will add the generated htpasswd in the input secret. The authentication name will be
// of the same name as the secret. If an identity provider of the same name is found, no action is
// taken.
func addHtPasswd(dynamicClient dynamic.Interface, secretName string) error {
	const oAuthName = "cluster"
	clusterOAuth, err := getClusterOAuthConfig(dynamicClient)
	if err != nil {
		return err
	}

	oauthRsrc := dynamicClient.Resource(GvrOAuth)
	// The type was already validated in getClusterOAuthConfig
	spec, _ := clusterOAuth.Object["spec"].(map[string]interface{})

	// If spec.identityProviders is not set, it needs to first be set to an empty array for the
	// patch below to work.
	if _, ok := spec["identityProviders"]; !ok {
		emptyArrayPatchObj := []k8sJSONPatch{
			{
				Op:    "add",
				Path:  "/spec/identityProviders",
				Value: []interface{}{},
			},
		}
		emptyArrayPatch, err := json.Marshal(emptyArrayPatchObj)
		if err != nil {
			return fmt.Errorf("failed to marshal the empty array patch to JSON: %w", err)
		}

		_, err = oauthRsrc.Patch(
			context.TODO(), oAuthName, types.JSONPatchType, emptyArrayPatch, metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf(`failed to patch the "%s" OAuth object: %w`, oAuthName, err)
		}
	} else {
		idps, ok := spec["identityProviders"].([]interface{})
		if !ok {
			return fmt.Errorf(
				`the "%s" OAuth object has an invalid spec.identityProviders field`, oAuthName,
			)
		}

		for i, idp := range idps {
			idp, ok := idp.(map[string]interface{})
			if !ok {
				return fmt.Errorf(
					`the "%s" OAuth object has an invalid spec.identityProviders[%d] field`,
					oAuthName,
					i,
				)
			}

			if name, _, _ := unstructured.NestedString(idp, "name"); name == secretName {
				// An identity provider of the same name already exists, so assume it it is correct.
				return nil
			}
		}
	}

	identityPatchObj := []k8sJSONPatch{
		{
			Op:   "add",
			Path: "/spec/identityProviders/-",
			Value: map[string]interface{}{
				"name":          secretName,
				"mappingMethod": "claim",
				"type":          "HTPasswd",
				"htpasswd": map[string]interface{}{
					"fileData": map[string]interface{}{
						"name": secretName,
					},
				},
			},
		},
	}
	identityPatch, err := json.Marshal(identityPatchObj)
	if err != nil {
		return fmt.Errorf("failed to marshal the identity provider patch to JSON: %w", err)
	}

	_, err = oauthRsrc.Patch(context.TODO(), oAuthName, types.JSONPatchType, identityPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf(
			`failed to patch the "%s" OAuth object spec.identityProviders field: %w`,
			oAuthName,
			err,
		)
	}

	return nil
}

// getClusterOAuthConfig gets the "cluster" OAuth object, which is used for identity provider
// configuration.
func getClusterOAuthConfig(dynamicClient dynamic.Interface) (*unstructured.Unstructured, error) {
	oauthRsrc := dynamicClient.Resource(GvrOAuth)
	const oAuthName = "cluster"
	clusterOAuth, err := oauthRsrc.Get(context.TODO(), oAuthName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf(`failed to get the "%s" OAuth object: %w`, oAuthName, err)
	}

	_, ok := clusterOAuth.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`the "%s" OAuth object has an invalid spec`, oAuthName)
	}

	return clusterOAuth, nil
}

// addClusterRoleBindings will add the user to the desired cluster role bindings without removing
// existing subjects. If the bindings are already set, nothing will occur.
func addClusterRoleBindings(client kubernetes.Interface, user OCPUser) error {
	for _, binding := range user.ClusterRoleBindings {
		bindingObj, err := client.RbacV1().ClusterRoleBindings().Get(
			context.TODO(), binding, metav1.GetOptions{},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to get the cluster role binding %s: %w", binding, err,
			)
		}

		alreadySet := false
		for _, subject := range bindingObj.Subjects {
			if subject.APIGroup != "rbac.authorization.k8s.io" || subject.Kind != "User" {
				continue
			}

			if subject.Name == user.Username {
				alreadySet = true
				break
			}
		}

		if alreadySet {
			continue
		}

		subject := map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "User",
			"name":     user.Username,
		}

		var patchObj interface{}
		var patchType types.PatchType
		// A strategic merge patch is required when there are no subjects since the Kubernetes API
		// rejects a JSON patch when there are no subjects set. Setting it first to an empty array
		// also does not work since Kubernetes sinces to discard the empty array. The strategic
		// merge patch works in this case, however, it does not work for the case when subjects
		// is already set to one or more values. The Kubernetes API will just overwrite the
		// entire subjects array in this case. This is why both patch types must be used.
		if len(bindingObj.Subjects) == 0 {
			patchType = types.StrategicMergePatchType
			patchObj = map[string]interface{}{
				"subjects": []map[string]interface{}{subject},
			}
		} else {
			patchType = types.JSONPatchType
			patchObj = []k8sJSONPatch{
				{
					Op:    "add",
					Path:  "/subjects/-",
					Value: subject,
				},
			}
		}

		patch, err := json.Marshal(patchObj)
		if err != nil {
			return fmt.Errorf(
				"failed to marshal the cluster role binding patch to JSON for %s: %w",
				binding,
				err,
			)
		}

		_, err = client.RbacV1().ClusterRoleBindings().Patch(
			context.TODO(), binding, patchType, patch, metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to patch the cluster role binding %s: %w", binding, err,
			)
		}
	}

	return nil
}

// addClusterRoles will create role bindings/cluster role bindings named in the format of `<username>-<role>`,
// for the user with the desired cluster role. If no namespace is provided, cluster role bindings will be
// created instead of a role binding. If the binding already exists by the name, no action will be taken.
func addClusterRoles(client kubernetes.Interface, user OCPUser) error {
	for _, role := range user.ClusterRoles {
		bindingName := user.Username + "-" + role.Name
		var err error

		if role.Namespace == "" {
			_, err = client.RbacV1().ClusterRoleBindings().Get(context.TODO(), bindingName, metav1.GetOptions{})
		} else {
			_, err = client.RbacV1().RoleBindings(role.Namespace).Get(context.TODO(), bindingName, metav1.GetOptions{})
		}

		if err == nil {
			// Assume this is correct and skip creating the binding
			continue
		}

		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the cluster role binding of %s: %w", bindingName, err)
		}

		roleRef := rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     role.Name,
		}
		subjectObjs := []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     user.Username,
			},
		}

		if role.Namespace == "" {
			binding := rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: bindingName},
				RoleRef:    roleRef,
				Subjects:   subjectObjs,
			}
			_, err = client.RbacV1().ClusterRoleBindings().Create(
				context.TODO(), &binding, metav1.CreateOptions{},
			)
		} else {
			binding := rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: bindingName},
				RoleRef:    roleRef,
				Subjects:   subjectObjs,
			}
			_, err = client.RbacV1().RoleBindings(role.Namespace).Create(
				context.TODO(), &binding, metav1.CreateOptions{},
			)
		}

		if err != nil {
			return fmt.Errorf("failed to create the binding of %s: %w", bindingName, err)
		}
	}

	return nil
}

// CleanupOCPUser will revert changes made to the cluster by the CreateOCPUser function.
func CleanupOCPUser(
	client kubernetes.Interface, dynamicClient dynamic.Interface, secretName string, user OCPUser,
) error {
	err := deleteHtPasswd(dynamicClient, secretName, user)
	if err != nil {
		return fmt.Errorf(
			"failed to delete the OpenShift identity provider for the associated secret %s: %w",
			secretName,
			err,
		)
	}

	err = client.CoreV1().Secrets(ocpConfigNs).Delete(
		context.TODO(), secretName, metav1.DeleteOptions{},
	)

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete the secret %s/%s: %w", ocpConfigNs, secretName, err)
	}

	err = removeClusterRoleBindings(client, user)
	if err != nil {
		return err
	}

	return removeClusterRoles(client, user)
}

// deleteHtPasswd deletes the htpasswd identity provider configuration entry of the input name and
// deletes the User and Identity objects created by OpenShift.
func deleteHtPasswd(dynamicClient dynamic.Interface, authName string, user OCPUser) error {
	const oAuthName = "cluster"
	clusterOAuth, err := getClusterOAuthConfig(dynamicClient)
	if err != nil {
		return err
	}

	spec, ok := clusterOAuth.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf(`the "%s" OAuth object has an invalid spec`, oAuthName)
	}

	if _, ok := spec["identityProviders"]; !ok {
		return nil
	}

	idps, ok := spec["identityProviders"].([]interface{})
	if !ok {
		return fmt.Errorf(
			`the "%s" OAuth object has an invalid spec.identityProviders field`, oAuthName,
		)
	}

	idpIndex := -1
	for i, idp := range idps {
		idp, ok := idp.(map[string]interface{})
		if !ok {
			return fmt.Errorf(
				`the "%s" OAuth object has an invalid spec.identityProviders[%d] field`,
				oAuthName,
				i,
			)
		}

		if name, _, _ := unstructured.NestedString(idp, "name"); name == authName {
			// An identity provider of the same name already exists, so assume it it is the one
			// we are looking for.
			idpIndex = i
			break
		}
	}

	if idpIndex < 0 {
		// The identity provider was not found, so no action is needed.
		return nil
	}

	identityPatchObj := []k8sJSONPatch{
		{
			Op:   "remove",
			Path: fmt.Sprintf("/spec/identityProviders/%d", idpIndex),
		},
	}
	identityPatch, err := json.Marshal(identityPatchObj)
	if err != nil {
		return fmt.Errorf("failed to marshal the identity provider removal patch to JSON: %w", err)
	}

	oauthRsrc := dynamicClient.Resource(GvrOAuth)
	_, err = oauthRsrc.Patch(context.TODO(), oAuthName, types.JSONPatchType, identityPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf(
			`failed to remove the "%s" OAuth object spec.identityProviders[%d] entry: %w`,
			oAuthName,
			idpIndex,
			err,
		)
	}

	// Delete the User and Identity objects created by OpenShift
	err = dynamicClient.Resource(GvrUser).Delete(
		context.TODO(), user.Username, metav1.DeleteOptions{},
	)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf(
			`failed to delete the OpenShift "User" of "%s": %w`, user.Username, err,
		)
	}

	identityName := fmt.Sprintf("%s-user:%s", user.Username, user.Username)
	err = dynamicClient.Resource(GvrIdentity).Delete(
		context.TODO(), identityName, metav1.DeleteOptions{},
	)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf(
			`failed to delete the OpenShift "Identity" of "%s": %w`, identityName, err,
		)
	}

	return nil
}

// removeClusterRoleBindings will remove the user from the desired cluster role bindings. No other
// subjects will be removed.
func removeClusterRoleBindings(client kubernetes.Interface, user OCPUser) error {
	for _, binding := range user.ClusterRoleBindings {
		bindingObj, err := client.RbacV1().ClusterRoleBindings().Get(
			context.TODO(), binding, metav1.GetOptions{},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to get the cluster role binding %s: %w", binding, err,
			)
		}

		subjectIndex := -1
		for i, subject := range bindingObj.Subjects {
			if subject.APIGroup != "rbac.authorization.k8s.io" || subject.Kind != "User" {
				continue
			}

			if subject.Name == user.Username {
				subjectIndex = i
				break
			}
		}

		if subjectIndex == -1 {
			continue
		}

		patchObj := []k8sJSONPatch{
			{
				Op:   "remove",
				Path: fmt.Sprintf("/subjects/%d", subjectIndex),
				Value: map[string]interface{}{
					"apiGroup": "rbac.authorization.k8s.io",
					"kind":     "User",
					"name":     user.Username,
				},
			},
		}
		patch, err := json.Marshal(patchObj)
		if err != nil {
			return fmt.Errorf(
				"failed to marshal the cluster role binding delete patch to JSON for %s: %w",
				binding,
				err,
			)
		}

		_, err = client.RbacV1().ClusterRoleBindings().Patch(
			context.TODO(), binding, types.JSONPatchType, patch, metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to delete the user from the cluster role binding %s: %w", binding, err,
			)
		}
	}

	return nil
}

// removeClusterRoles will delete the generated cluster role bindings for the user that were
// created in addClusterRoles.
func removeClusterRoles(client kubernetes.Interface, user OCPUser) error {
	for _, role := range user.ClusterRoles {
		bindingName := user.Username + "-" + role.Name
		var err error

		if role.Namespace == "" {
			err = client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), bindingName, metav1.DeleteOptions{})
		} else {
			err = client.RbacV1().RoleBindings(role.Namespace).Delete(
				context.TODO(), bindingName, metav1.DeleteOptions{},
			)
		}

		if err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete the binding of %s: %w", bindingName, err)
		}
	}

	return nil
}
