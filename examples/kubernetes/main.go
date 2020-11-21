package foo

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/pkg/errors"
	kwhhttp "github.com/slok/kubewebhook/pkg/http"
	kwhlog "github.com/slok/kubewebhook/pkg/log"
	kwhmutating "github.com/slok/kubewebhook/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// showDebugMsgsAnnotKey is the name of a Pod Annotation key which
	// indicates whether the debug mesages should be shown or not.
	showDebugMsgsAnnotKey = "berglas-show-debug-messages"

	// disabledAnnotKey is the name of a Pod Annotations key which
	// explicitly disables Berglas for that Pod.
	berglasEnabledAnnotKey = "berglas-enabled"

	// containersSelectedAnnotKey is the name of a Pod Annotation key which
	// specifies which init containers should only be selected for
	// processing.
	icontainersSelectedAnnotKey = "berglas-init-containers-selected"

	// containersIgnoredAnnotKey is the name of a Pod Annotation key which
	// specifies which init containers should be excluded from processing.
	icontainersIgnoredAnnotKey = "berglas-init-containers-ignored"

	// containersSelectedAnnotKey is the name of a Pod Annotation key which
	// specifies which containers should only be selected for processing.
	containersSelectedAnnotKey = "berglas-containers-selected"

	// containersIgnoredAnnotKey is the name of a Pod Annotation key which
	// specifies which containers should be excluded from processing.
	containersIgnoredAnnotKey = "berglas-containers-ignored"

	// queryRegistryAnnotKey is the name of a Pod Annotations key which
	// indicates whether to try to get command from remote registry if not
	// defined in the Pod.
	queryRegistryAnnotKey = "berglas-query-registry"

	// secretsEnabledAnnotKey is the name of a Pod Annotation key which
	// indicates wherther the secrets should be processed or not.
	secretsEnabledAnnotKey = "berglas-secrets-enabled"

	// secretsIgnoreSaAnnotKey is the name of a Pod Annotation key which
	// indicates whether the Service Account secrets should be ignored or
	// not.
	secretsIgnoreSaAnnotKey = "berglas-secrets-ignore-sa"

	// volumesSelectedAnnotKey is the name of a Pod Annotation key which
	// specifies which Volume names that reference secrets should only be
	// selected for processing.
	volumesSelectedAnnotKey = "berglas-secrets-volumes-selected"

	// volumesIgnoredAnnotKey is the name of a Pod Annotation key which
	// specifies which Volume names that reference secrets should be
	// excluded from processing.
	volumesIgnoredAnnotKey = "berglas-secrets-volumes-ignored"

	// secretsExecUserAnnotKey is the name of a Pod Annotation key which
	// specifies user and group under which Berglas should execute the
	// container's command.
	secretsExecUserAnnotKey = "berglas-secrets-exec-user"

	// secretsRunAsUserAnnotKey is the name of a Pod Annotation key which
	// specifies user ID that will be used for the runAsUser in the
	// container securityContext.
	secretsRunAsUserAnnotKey = "berglas-secrets-run-as-user"

	// secretsRunAsGroupAnnotKey is the name of a Pod Annotation key which
	// specifies user ID that will be used for the runAsGroup in the
	// container securityContext.
	secretsRunAsGroupAnnotKey = "berglas-secrets-run-as-group"

	// secretsRunAsNonRootAnnotKey is the name of a Pod Annotation key
	// which indicates whether to runAsNonRoot should be enabled or not in
	// the container securityContext.
	secretsRunAsNonRootAnnotKey = "berglas-secrets-run-as-non-root"

	// secretsAllowPrivEscal is the name of a Pod Annotation key which
	// indicates whether the secretsAllowPrivilegeEscalation should be
	// enabled or not in the container securityContext.
	secretsAllowPrivEscalAnnotKey = "berglas-secrets-allow-privilege-escalation"

	// secretsRoRootFsEnabledAnnotKey is the name of a Pod Annotation key
	// which indicates whether the readOnlyRootFilesystem should be enabled
	// or not in the container securityContext.
	secretsRoRootFsEnabledAnnotKey = "berglas-secrets-rorf-enabled"

	// berglasContainer is the default berglas container from which to pull the
	// berglas binary.
	// TODO: berglasContainer = "us-docker.pkg.dev/berglas/berglas/berglas:latest"
	berglasContainer = "jtyr/berglas:v1.5.0"

	// binVolumeName is the name of the volume where the berglas binary is stored.
	binVolumeName = "berglas-bin"

	// binVolumeMountPath is the mount path where the berglas binary can be found.
	binVolumeMountPath = "/berglas/bin/"

	// saSecretPath is a path which Kubernetes uses to store Service Accout
	// secrets inside the container.
	saSecretPath = "/var/run/secrets/kubernetes.io/serviceaccount"

	// secretMountPathPostfix is a postfix added to every secret mountPath.
	// TODO: secretMountPathPostfix = berglas.SecretsMountPathPostfix
	secretMountPathPostfix = "..berglas"

	// envSecretsPaths is the name of the env var which will hold list of
	// files and directoris for Berglas to explore.
	// TODO: envSecretsPaths = berglas.SecretsPathsEnvVarName
	envSecretsPaths = "BERGLAS_SECRETS_PATHS"

	// envExecUser is the name of the env var which will hold list of files
	// and directoris for Berglas to explore.
	// TODO: envExecUser = berglas.SecretsUserExecEnvVarName
	envExecUser = "BERGLAS_SECRETS_EXEC_USER"
)

var (
	// showDebugMsgs indicates whether to show debug messages by default.
	showDebugMsgs = true

	// berglasEnabled indicates whether the mutator should do anything by
	// default.
	berglasEnabled = true

	// queryRegistry indicates whether to try to get command from remote
	// registry by default if not defined in the Pod.
	queryRegistry = true

	// registryTimeout indicates how many seconds to wait before the request
	// fails.
	registryTimeout = 3

	// secretsEnabled indicates whether to process secrets by default.
	secretsEnabled = false

	// secretsIgnoreSa indicates whether the Service Account secrets should
	// be ignored by default.
	secretsIgnoreSa = true
)

// binInitContainer is the container that pulls the berglas binary executable
// into a shared volume mount.
var binInitContainer = corev1.Container{
	Name:            "copy-berglas-bin",
	Image:           berglasContainer,
	ImagePullPolicy: corev1.PullIfNotPresent,
	Command: []string{"sh", "-c",
		fmt.Sprintf("cp /bin/berglas %s", binVolumeMountPath)},
	VolumeMounts: []corev1.VolumeMount{
		{
			Name:      binVolumeName,
			MountPath: binVolumeMountPath,
		},
	},
}

// binVolume is the shared, in-memory volume where the berglas binary lives.
var binVolume = corev1.Volume{
	Name: binVolumeName,
	VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		},
	},
}

// binVolumeMount is the shared volume mount where the berglas binary lives.
var binVolumeMount = corev1.VolumeMount{
	Name:      binVolumeName,
	MountPath: binVolumeMountPath,
	ReadOnly:  true,
}

// Config is a configuration for the mutator.
type Config struct {
	showDebugMsgs          bool
	berglasEnabled         bool
	icontainersSelected    []string
	icontainersIgnored     []string
	containersSelected     []string
	containersIgnored      []string
	queryRegistry          bool
	secretsEnabled         bool
	secretsIgnoreSa        bool
	volumesSelected        []string
	volumesIgnored         []string
	secretsExecUser        string
	secretsRunAsUser       *int64
	secretsRunAsGroup      *int64
	secretsRunAsNonRoot    *bool
	secretsAllowPrivEscal  *bool
	secretsRoRootFsEnabled *bool
}

// BerglasMutator is a mutator.
type BerglasMutator struct {
	logger kwhlog.Logger
	config Config
}

// VolumesSecrets is used to store information about Volume secrets.
type VolumesSecrets map[string][]string

// DockerAuth is used for getting Docker registry access token.
type DockerAuth struct {
	Token string `json:"token"`
}

// DockerMeta is describing Docker registry metadata.
type DockerMeta struct {
	Config DockerMetaConfig `json:"config"`
}

// DockerMeta is describing configuration from the Docker registry metadata.
type DockerMetaConfig struct {
	Digest string `json:"digest"`
}

// DockerMeta is describing Docker registry blob.
type DockerBlob struct {
	Config DockerBlobConfig `json:"config"`
}

// DockerMeta is describing container config from the Docker registry blob.
type DockerBlobConfig struct {
	Entrypoint []string `json:"Entrypoint"`
	Cmd        []string `json:"Cmd"`
	User       string   `json:"User"`
}

// Mutate implements MutateFunc and provides the top-level entrypoint for object
// mutation.
func (m *BerglasMutator) Mutate(ctx context.Context, obj metav1.Object) (bool, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		m.logger.Errorf("not a pod")
		return false, nil
	}

	// Check Pod Annotations if berlas should be disable.
	annot := pod.ObjectMeta.GetAnnotations()

	// Set config options.
	m.setConfig(annot)

	// Set logger.
	m.logger = &kwhlog.Std{Debug: m.config.showDebugMsgs}

	m.logger.Debugf("calling Mutate")

	// Check if berglas is enabled.
	if !m.config.berglasEnabled {
		m.logger.Debugf("berglas explicity disabled")
		return false, nil
	}

	// Check for Volumes with secret reference.
	vs := make(VolumesSecrets)

	for _, v := range pod.Spec.Volumes {
		if v.VolumeSource.Secret != nil {
			if valueInArray(m.config.volumesSelected, v.Name, true) &&
				!valueInArray(m.config.volumesIgnored, v.Name, false) {

				vs[v.Name] = nil

				for _, i := range v.VolumeSource.Secret.Items {
					vs[v.Name] = append(vs[v.Name], i.Path)
				}
			}
		}
	}

	mutated := false

	for i, c := range pod.Spec.InitContainers {
		if valueInArray(m.config.icontainersSelected, c.Name, true) &&
			!valueInArray(m.config.icontainersIgnored, c.Name, false) {

			m.logger.Debugf("mutating init container: %s", c.Name)
			c, didMutate := m.mutateContainer(ctx, &c, vs)
			if didMutate {
				mutated = true
				pod.Spec.InitContainers[i] = *c
			}
		} else {
			m.logger.Debugf("ignoring init container: %s", c.Name)
		}
	}

	for i, c := range pod.Spec.Containers {
		if valueInArray(m.config.containersSelected, c.Name, true) &&
			!valueInArray(m.config.containersIgnored, c.Name, false) {

			m.logger.Debugf("mutating container: %s", c.Name)
			c, didMutate := m.mutateContainer(ctx, &c, vs)
			if didMutate {
				mutated = true
				pod.Spec.Containers[i] = *c
			}
		} else {
			m.logger.Debugf("ignoring container: %s", c.Name)
		}
	}

	// If any of the containers requested berglas secrets, mount the shared volume
	// and ensure the berglas binary is available via an init container.
	if mutated {
		m.logger.Debugf("adding berglas initContainer and volume")
		pod.Spec.Volumes = append(pod.Spec.Volumes, binVolume)
		pod.Spec.InitContainers = append([]corev1.Container{binInitContainer},
			pod.Spec.InitContainers...)
	}

	return false, nil
}

// mutateContainer mutates the given container, updating the volume mounts and
// command if it contains berglas references.
func (m *BerglasMutator) mutateContainer(_ context.Context, c *corev1.Container, vs VolumesSecrets) (*corev1.Container, bool) {
	var metadata DockerBlobConfig
	var command = c.Command

	if m.config.queryRegistry {
		meta, err := m.fetchMetaFromRegistry(c.Image)
		if err != nil {
			m.logger.Errorf("failed to fetch metadata from registry: %s", err)
		}

		metadata = meta

		if m.config.secretsEnabled && metadata.User != "" {
			m.logger.Debugf("setting user from the registry")
			// Overwrite the exec user settings
			m.config.secretsExecUser = metadata.User
			// Force the container to run as root
			intZero := int64(0)
			falseBool := false
			m.config.secretsRunAsUser = &intZero
			m.config.secretsRunAsGroup = &intZero
			m.config.secretsRunAsNonRoot = &falseBool
		}
	}

	// Berglas prepends the command from the podspec. If there's no command in the
	// podspec, there's nothing to append. Note: this is the command in the
	// podspec, not a CMD or ENTRYPOINT in a Dockerfile. We will try to get
	// the command from the image metadata in the registry (see the
	// queryRegistry config option).
	if len(command) == 0 {
		if metadata.Entrypoint != nil {
			m.logger.Debugf("setting command to be the entrypoint from registry")
			command = metadata.Entrypoint
		} else if metadata.Cmd != nil {
			m.logger.Debugf("setting command to be the cmd from registry")
			command = metadata.Cmd
		}

		if command == nil {
			m.logger.Warningf("cannot apply berglas to %s: container spec does not define a command", c.Name)
			return c, false
		}
	}

	var ms []string

	// Compile list of paths to secrets.
	if m.config.secretsEnabled {
		for i, vm := range c.VolumeMounts {
			if _, ok := vs[vm.Name]; ok {
				if secretsIgnoreSa && vm.MountPath == saSecretPath {
					m.logger.Debugf("ignoring SA secret: %s", vm.Name)
				} else {
					mountPath := filepath.Clean(vm.MountPath)
					newMountPath := fmt.Sprintf("%s/%s", mountPath, secretMountPathPostfix)
					m.logger.Debugf("mutating volumeMount[name=%s] to be %s", vm.Name, newMountPath)
					c.VolumeMounts[i].MountPath = newMountPath

					if len(vs[vm.Name]) > 0 {
						for _, p := range vs[vm.Name] {
							ms = append(ms, fmt.Sprintf("%s//%s", mountPath, filepath.Clean(p)))
						}
					} else {
						ms = append(ms, vm.MountPath)
					}
				}
			}
		}
	}

	if !m.hasBerglasReferences(c.Env) && !m.hasEnvSecret(c.Env) && !m.hasEnvFromSecret(c.EnvFrom) && len(ms) == 0 {
		// Ignore if there are no berglas references or no reference to
		// a secret in the container.
		m.logger.Debugf("no env or secret found")
		return c, false
	}

	// Add the shared volume mount
	c.VolumeMounts = append(c.VolumeMounts, binVolumeMount)

	// Prepend the command with berglas exec --
	original := append(command, c.Args...)
	c.Command = []string{binVolumeMountPath + "berglas"}
	c.Args = append([]string{"exec", "--"}, original...)

	if len(ms) > 0 {
		evPaths := corev1.EnvVar{
			Name:  envSecretsPaths,
			Value: strings.Join(ms, ","),
		}

		m.logger.Debugf("adding env var: %s=%s", evPaths.Name, evPaths.Value)
		c.Env = append(c.Env, evPaths)
	}

	// Add exec user
	if m.config.secretsExecUser != "" {
		evUser := corev1.EnvVar{
			Name:  envExecUser,
			Value: m.config.secretsExecUser,
		}

		m.logger.Debugf("adding env var: %s=%s", evUser.Name, evUser.Value)
		c.Env = append(c.Env, evUser)
	}

	// Enforce security context
	if c.SecurityContext == nil && (m.config.secretsRunAsUser != nil ||
		m.config.secretsRunAsGroup != nil ||
		m.config.secretsRunAsNonRoot != nil ||
		m.config.secretsAllowPrivEscal != nil ||
		m.config.secretsRoRootFsEnabled != nil) {

		m.logger.Debugf("adding security context")
		c.SecurityContext = &corev1.SecurityContext{}
	}
	if m.config.secretsRunAsUser != nil {
		m.logger.Debugf("setting securityContext.runAsUser=%d", *m.config.secretsRunAsUser)
		c.SecurityContext.RunAsUser = m.config.secretsRunAsUser
	}
	if m.config.secretsRunAsGroup != nil {
		m.logger.Debugf("setting securityContext.runAsGroup=%d", *m.config.secretsRunAsGroup)
		c.SecurityContext.RunAsGroup = m.config.secretsRunAsGroup
	}
	if m.config.secretsRunAsNonRoot != nil {
		m.logger.Debugf("setting securityContext.runAsNonRoot=%t", *m.config.secretsRunAsNonRoot)
		c.SecurityContext.RunAsNonRoot = m.config.secretsRunAsNonRoot
	}
	if m.config.secretsAllowPrivEscal != nil {
		m.logger.Debugf("setting securityContext.allowPrivilegeEscalation=%t", *m.config.secretsAllowPrivEscal)
		c.SecurityContext.AllowPrivilegeEscalation = m.config.secretsAllowPrivEscal
	}
	if m.config.secretsRoRootFsEnabled != nil {
		m.logger.Debugf("setting securityContext.readOnlyRootFilesystem=%t", *m.config.secretsRoRootFsEnabled)
		c.SecurityContext.ReadOnlyRootFilesystem = m.config.secretsRoRootFsEnabled
	}

	return c, true
}

// Set config options based on default values or values from Annotations.
func (m *BerglasMutator) setConfig(annot map[string]string) {
	// Global
	m.config.showDebugMsgs = *getBoolAnnot(annot, showDebugMsgsAnnotKey, &showDebugMsgs)
	m.config.berglasEnabled = *getBoolAnnot(annot, berglasEnabledAnnotKey, &berglasEnabled)
	m.config.icontainersSelected = getStringArrayAnnot(annot, icontainersSelectedAnnotKey)
	m.config.icontainersIgnored = getStringArrayAnnot(annot, icontainersIgnoredAnnotKey)
	m.config.containersSelected = getStringArrayAnnot(annot, containersSelectedAnnotKey)
	m.config.containersIgnored = getStringArrayAnnot(annot, containersIgnoredAnnotKey)
	m.config.queryRegistry = *getBoolAnnot(annot, queryRegistryAnnotKey, &queryRegistry)
	m.config.secretsEnabled = *getBoolAnnot(annot, secretsEnabledAnnotKey, &secretsEnabled)
	// TODO: Per container (val@container1,val@container2)?
	m.config.secretsIgnoreSa = *getBoolAnnot(annot, secretsIgnoreSaAnnotKey, &secretsIgnoreSa)
	m.config.volumesSelected = getStringArrayAnnot(annot, volumesSelectedAnnotKey)
	m.config.volumesIgnored = getStringArrayAnnot(annot, volumesIgnoredAnnotKey)
	m.config.secretsExecUser = getStringAnnot(annot, secretsExecUserAnnotKey)
	m.config.secretsRunAsUser = getInt64Annot(annot, secretsRunAsUserAnnotKey, nil)
	m.config.secretsRunAsGroup = getInt64Annot(annot, secretsRunAsGroupAnnotKey, nil)
	m.config.secretsRunAsNonRoot = getBoolAnnot(annot, secretsRunAsNonRootAnnotKey, nil)
	m.config.secretsAllowPrivEscal = getBoolAnnot(annot, secretsAllowPrivEscalAnnotKey, nil)
	m.config.secretsRoRootFsEnabled = getBoolAnnot(annot, secretsRoRootFsEnabledAnnotKey, nil)
}

// hasBerglasReferences parses the environment and returns true if any of the
// environment variables includes a berglas reference.
func (m *BerglasMutator) hasBerglasReferences(env []corev1.EnvVar) bool {
	for _, e := range env {
		if berglas.IsReference(e.Value) {
			m.logger.Debugf("found berglas reference in env")
			return true
		}
	}

	return false
}

// hasEnvSecret parses the environment and returns true if any of the
// environment variables is a reference to a secret.
func (m *BerglasMutator) hasEnvSecret(env []corev1.EnvVar) bool {
	if !m.config.secretsEnabled {
		return false
	}

	for _, e := range env {
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			m.logger.Debugf("found env secret")
			return true
		}
	}

	return false
}

// hasEnvFromSecret parses the environment from and returns true if any of the
// environment variables is a reference to a secret.
func (m *BerglasMutator) hasEnvFromSecret(env []corev1.EnvFromSource) bool {
	if !m.config.secretsEnabled {
		return false
	}

	for _, e := range env {
		if e.SecretRef != nil {
			m.logger.Debugf("found envFrom secret")
			return true
		}
	}

	return false
}

// Fetch image metadata from remote registry.
func (m *BerglasMutator) fetchMetaFromRegistry(img string) (DockerBlobConfig, error) {
	var data DockerBlobConfig

	// Normalize the image name
	if strings.Index(img, "/") == -1 {
		img = fmt.Sprintf("library/%s", img)
	}

	parts := strings.Split(img, "/")

	var isDockerHub bool
	if len(parts) == 1 || len(parts) == 2 {
		isDockerHub = true
	}

	var registry string
	var imgPath string
	var imgTag string

	// Determine or set image tag
	lastPart := len(parts) - 1
	imgTagParts := strings.Split(parts[lastPart], ":")
	if len(imgTagParts) == 1 {
		imgTag = "latest"
	} else {
		parts[lastPart] = imgTagParts[0]
		imgTag = imgTagParts[1]
	}

	// Set registry and image path
	if isDockerHub {
		registry = "registry-1.docker.io"
		imgPath = strings.Join(parts, "/")
	} else {
		registry = parts[0]
		imgPath = strings.Join(parts[1:], "/")
	}

	// Get token
	header := make(http.Header)
	if isDockerHub {
		tokenBody, err := m.fetchJson(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", imgPath), nil)
		if err != nil {
			return data, errors.Wrapf(err, "failed to fetch token data")
		}

		var tokenData DockerAuth
		if err := json.Unmarshal(tokenBody, &tokenData); err != nil {
			return data, errors.Wrapf(err, "failed to parse token JSON data")
		}

		if tokenData.Token != "" {
			header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenData.Token))
		} else {
			return data, errors.Wrapf(err, "invalid token data received: %s", tokenBody)
		}
	}

	// Request image digest
	header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	digestBody, err := m.fetchJson(fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, imgPath, imgTag), header)
	if err != nil {
		return data, errors.Wrapf(err, "failed to fetch digest data")
	}

	var digestData DockerMeta
	if err := json.Unmarshal(digestBody, &digestData); err != nil {
		return data, errors.Wrapf(err, "failed to parse digest JSON data")
	}

	// Request final metadata
	if digestData.Config.Digest == "" {
		return data, errors.Wrapf(err, "invalid digest data received: %s", digestBody)
	} else {
		blobBody, err := m.fetchJson(fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, imgPath, digestData.Config.Digest), header)
		if err != nil {
			return data, errors.Wrapf(err, "failed to fetch blob data")
		}

		var blobData DockerBlob
		if err := json.Unmarshal(blobBody, &blobData); err != nil {
			return data, errors.Wrapf(err, "failed to parse blob JSON data")
		}

		if blobData.Config.Entrypoint == nil && blobData.Config.Cmd == nil {
			return data, errors.Wrapf(err, "invalid blob data received: %s", blobBody)
		} else {
			data = blobData.Config
		}
	}

	return data, nil
}

// Fetch JSON from remote API.
func (m *BerglasMutator) fetchJson(url string, header http.Header) ([]byte, error) {
	var data []byte

	client := http.Client{
		Timeout: time.Second * time.Duration(registryTimeout),
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return data, errors.Wrapf(err, "failed to build request")
	}

	if header != nil {
		req.Header = header
	}

	resp, getErr := client.Do(req)
	if getErr != nil {
		return data, errors.Wrapf(getErr, "failed to execute request")
	}

	if resp.StatusCode != 200 {
		return data, errors.Wrapf(errors.New(resp.Status), "incorrect status code returned")
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return data, errors.Wrapf(readErr, "failed to read response body")
	}

	data = body

	return data, nil
}

// Return the annotation's or the default bool value.
func getBoolAnnot(annot map[string]string, key string, defVal *bool) *bool {
	if val, ok := annot[key]; ok {
		bVal := false

		if val == "yes" || val == "true" {
			bVal = true
		}

		return &bVal
	}

	return defVal
}

// Return the annotation's or the default int64 value.
func getInt64Annot(annot map[string]string, key string, defVal *int64) *int64 {
	if val, ok := annot[key]; ok {
		if n, err := strconv.Atoi(val); err == nil {
			iVal := int64(n)
			return &iVal
		}
	}

	return defVal
}

// Return the annotation's value splitted into array of strings.
func getStringArrayAnnot(annot map[string]string, key string) []string {
	if val, ok := annot[key]; ok {
		var vals []string

		for _, v := range strings.Split(val, ",") {
			vals = append(vals, strings.TrimSpace(v))
		}

		return vals
	}

	return []string{}
}

// Return the annotation's value.
func getStringAnnot(annot map[string]string, key string) string {
	if val, ok := annot[key]; ok {
		return val
	}

	return ""
}

// Return true if the string is in the array.
func valueInArray(a []string, s string, d bool) bool {
	if len(a) == 0 {
		return d
	}

	for _, v := range a {
		if v == s {
			return true
		}
	}

	return false
}

// webhookHandler is the http.Handler that responds to webhooks
func webhookHandler() http.Handler {
	logger := &kwhlog.Std{Debug: showDebugMsgs}

	mutator := &BerglasMutator{logger: logger}

	mcfg := kwhmutating.WebhookConfig{
		Name: "berglasSecrets",
		Obj:  &corev1.Pod{},
	}

	// Create the wrapping webhook
	wh, err := kwhmutating.NewWebhook(mcfg, mutator, nil, nil, logger)
	if err != nil {
		logger.Errorf("error creating webhook: %s", err)
		os.Exit(1)
	}

	// Get the handler for our webhook.
	whhandler, err := kwhhttp.HandlerFor(wh)
	if err != nil {
		logger.Errorf("error creating webhook handler: %s", err)
		os.Exit(1)
	}

	return whhandler
}

// F is the exported webhook for the function to bind.
var F = webhookHandler().ServeHTTP
