package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlog "github.com/slok/kubewebhook/v2/pkg/log"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhmutating "github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kubernetesConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// berglasContainer is the default berglas container from which to pull the
	// berglas binary.
	berglasContainer = "us-docker.pkg.dev/berglas/berglas/berglas:latest"

	// binVolumeName is the name of the volume where the berglas binary is stored.
	binVolumeName = "berglas-bin"

	// binVolumeMountPath is the mount path where the berglas binary can be found.
	binVolumeMountPath = "/berglas/bin/"
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
	Resources: corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("20m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
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

// BerglasMutator is a mutator.
type BerglasMutator struct {
	logger    kwhlog.Logger
	k8sClient kubernetes.Interface
}

func newK8SClient() (kubernetes.Interface, error) {
	kubeConfig, err := kubernetesConfig.GetConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func (m *BerglasMutator) getDataFromConfigmap(cmName string, ns string) (map[string]string, error) {
	configMap, err := m.k8sClient.CoreV1().ConfigMaps(ns).Get(context.Background(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return configMap.Data, nil
}

// Mutate implements MutateFunc and provides the top-level entrypoint for object
// mutation.
func (m *BerglasMutator) Mutate(ctx context.Context, ar *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhmutating.MutatorResult, error) {
	m.logger.Infof("calling mutate")
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return &kwhmutating.MutatorResult{
			Warnings: []string{fmt.Sprintf("incoming resource is not a Pod (%T)", pod)},
		}, nil
	}

	mutated := false
	isConfigMap := false
	for i, c := range pod.Spec.InitContainers {
		for _, ef := range pod.Spec.InitContainers[i].EnvFrom {
			if ef.ConfigMapRef != nil {
				data, err := m.getDataFromConfigmap(ef.ConfigMapRef.Name, ar.Namespace)
				if err != nil {
					if apierrors.IsNotFound(err) || (ef.ConfigMapRef.Optional != nil && *ef.ConfigMapRef.Optional) {
						continue
					}
				}
				for _, value := range data {
					if berglas.IsReference(value) {
						mutated = true
						isConfigMap = true
					}
				}
			}
		}
		c, didMutate := m.mutateContainer(ctx, &c, isConfigMap)
		if didMutate {
			mutated = true
			pod.Spec.InitContainers[i] = *c
		}
	}

	for i, c := range pod.Spec.Containers {
		for _, ef := range pod.Spec.Containers[i].EnvFrom {
			if ef.ConfigMapRef != nil {
				data, err := m.getDataFromConfigmap(ef.ConfigMapRef.Name, ar.Namespace)
				if err != nil {
					if apierrors.IsNotFound(err) || (ef.ConfigMapRef.Optional != nil && *ef.ConfigMapRef.Optional) {
						continue
					}
				}
				for _, value := range data {
					if berglas.IsReference(value) {
						mutated = true
						isConfigMap = true
					}
				}
			}
		}
		c, didMutate := m.mutateContainer(ctx, &c, isConfigMap)
		if didMutate {
			mutated = true
			pod.Spec.Containers[i] = *c
		}
	}

	// If any of the containers requested berglas secrets, mount the shared volume
	// and ensure the berglas binary is available via an init container.
	if mutated {
		pod.Spec.Volumes = append(pod.Spec.Volumes, binVolume)
		pod.Spec.InitContainers = append([]corev1.Container{binInitContainer},
			pod.Spec.InitContainers...)
	}

	return &kwhmutating.MutatorResult{
		MutatedObject: pod,
	}, nil
}

// mutateContainer mutates the given container, updating the volume mounts and
// command if it contains berglas references.
func (m *BerglasMutator) mutateContainer(_ context.Context, c *corev1.Container, isConfigMap bool) (*corev1.Container, bool) {
	// Ignore if there are no berglas references in the container.
	if !m.hasBerglasReferences(c.Env) && !isConfigMap {
		return c, false
	}

	// Berglas prepends the command from the podspec. If there's no command in the
	// podspec, there's nothing to append. Note: this is the command in the
	// podspec, not a CMD or ENTRYPOINT in a Dockerfile.
	if len(c.Command) == 0 {
		m.logger.Warningf("cannot apply berglas to %s: container spec does not define a command", c.Name)
		return c, false
	}

	// Add the shared volume mount
	c.VolumeMounts = append(c.VolumeMounts, binVolumeMount)

	// Prepend the command with berglas exec --
	original := append(c.Command, c.Args...)
	c.Command = []string{binVolumeMountPath + "berglas"}
	c.Args = append([]string{"exec", "--"}, original...)

	return c, true
}

// hasBerglasReferences parses the environment and returns true if any of the
// environment variables includes a berglas reference.
func (m *BerglasMutator) hasBerglasReferences(env []corev1.EnvVar) bool {
	for _, e := range env {
		if berglas.IsReference(e.Value) {
			return true
		}
	}
	return false
}

// healthCheckHandler is a simple health check handler.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK!")
}

// webhookHandler is the http.Handler that responds to webhooks
func webhookHandler(k8sClient kubernetes.Interface) (http.Handler, error) {
	entry := logrus.NewEntry(logrus.New())
	entry.Logger.SetLevel(logrus.DebugLevel)
	logger := kwhlogrus.NewLogrus(entry)

	mutator := &BerglasMutator{logger: logger, k8sClient: k8sClient}

	mcfg := kwhmutating.WebhookConfig{
		ID:      "berglasSecrets",
		Obj:     &corev1.Pod{},
		Mutator: mutator,
		Logger:  logger,
	}

	// Create the wrapping webhook
	wh, err := kwhmutating.NewWebhook(mcfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mutating webhook: %w", err)
	}

	// Create a new ServeMux to handle multiple routes
	mux := http.NewServeMux()

	// Register the health check handler at /healthcheck
	mux.HandleFunc("/healthcheck", healthCheckHandler)

	// Register the mutating webhook handler
	whhandler, err := kwhhttp.HandlerFor(kwhhttp.HandlerConfig{
		Webhook: wh,
		Logger:  logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mutating webhook handler: %w", err)
	}

	// Use the ServeMux to handle different routes
	mux.Handle("/mutate", whhandler)

	return mux, nil
}

func logAt(lvl, msg string, args ...interface{}) {
	body := map[string]interface{}{
		"time":     time.Now().UTC().Format(time.RFC3339),
		"severity": lvl,
		"message":  fmt.Sprintf(msg, args...),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		panic(fmt.Sprintf("failed to make JSON error message: %s", err))
	}
	fmt.Fprintln(os.Stderr, string(payload))
}

func logInfo(msg string, args ...interface{}) {
	logAt("INFO", msg, args...)
}

func logError(msg string, args ...interface{}) {
	logAt("ERROR", msg, args...)
}

func realMain() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8443"
	}

	// Load SSL certificate and private key from environment variables
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_PRIVATE_KEY_FILE")

	cert, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read SSL certificate: %w", err)
	}

	key, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("failed to read SSL key: %w", err)
	}

	// Parse the certificate and private key
	parsedCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return fmt.Errorf("failed to parse certificate and private key: %w", err)
	}

	// Create a TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{parsedCert},
	}

	k8sClient, err := newK8SClient()
	if err != nil {
		logError("failed to create Kubernetes client: %v", err)
		os.Exit(1)
	}

	handler, err := webhookHandler(k8sClient)
	if err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	srv := http.Server{
		Addr:      ":" + port,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	// Start the server
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeTLS("", "")
	}()

	logInfo("server is listening on " + port)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server failed to shutdown: %w", err)
	}

	// Wait for shutdown
	if err := <-errCh; err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func main() {
	if err := realMain(); err != nil {
		logError(err.Error())
		os.Exit(1)
	}

	logInfo("server successfully stopped")
}
