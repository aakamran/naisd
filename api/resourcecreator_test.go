package api

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
	"testing"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/resource"
)

const (
	appName         = "appname"
	otherAppName    = "otherappname"
	namespace       = "namespace"
	image           = "docker.hub/app"
	port            = 6900
	resourceVersion = "12369"
	version         = "13"
	livenessPath    = "isAlive"
	readinessPath   = "isReady"
	cpuRequest      = "100m"
	cpuLimit        = "200m"
	memoryRequest   = "200Mi"
	memoryLimit     = "400Mi"
	clusterIP       = "1.2.3.4"
)

func TestService(t *testing.T) {
	service := createOrUpdateServiceDef(port, nil, appName, namespace)
	service.ObjectMeta.ResourceVersion = resourceVersion
	service.Spec.ClusterIP = clusterIP
	clientset := fake.NewSimpleClientset(service)

	t.Run("Fetching nonexistant service yields nil and no error", func(t *testing.T) {
		nonExistantService, err := getExistingService("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistantService)
	})

	t.Run("Fetching an existing service yields service and no error", func(t *testing.T) {
		existingService, err := getExistingService(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, service, existingService)
	})

	t.Run("when no service exists, a new one is created", func(t *testing.T) {
		service, err := createOrUpdateService(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName, Version: version}, NaisAppConfig{Port: port}, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, service.ObjectMeta.Name)
		assert.Equal(t, int32(port), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": otherAppName}, service.Spec.Selector)
	})
	t.Run("when service exists, it's updated", func(t *testing.T) {
		newPort := 69
		service, err := createOrUpdateService(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, NaisAppConfig{Port: newPort}, clientset)
		assert.NoError(t, err)

		assert.Equal(t, resourceVersion, service.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName, service.ObjectMeta.Name)
		assert.Equal(t, int32(newPort), service.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, map[string]string{"app": appName}, service.Spec.Selector)
		assert.Equal(t, clusterIP, service.Spec.ClusterIP, "cluster ip is unchanged")
	})
}

func TestDeployment(t *testing.T) {
	newVersion := "14"
	resource1Name := "r1"
	resource1Type := "db"
	resource1Key := "key1"
	resource1Value := "value1"
	secret1Key := "password"
	secret1Value := "secret"

	resource2Name := "r2"
	resource2Type := "db"
	resource2Key := "key2"
	resource2Value := "value2"
	secret2Key := "password"
	secret2Value := "anothersecret"

	invalidlyNamedResourceNameDot := "dots.are.not.allowed"
	invalidlyNamedResourceTypeDot := "restservice"
	invalidlyNamedResourceKeyDot := "key"
	invalidlyNamedResourceValueDot := "value"
	invalidlyNamedResourceSecretKeyDot := "secretkey"
	invalidlyNamedResourceSecretValueDot := "secretvalue"

	invalidlyNamedResourceNameColon := "colon:are:not:allowed"
	invalidlyNamedResourceTypeColon := "restservice"
	invalidlyNamedResourceKeyColon := "key"
	invalidlyNamedResourceValueColon := "value"
	invalidlyNamedResourceSecretKeyColon := "secretkey"
	invalidlyNamedResourceSecretValueColon := "secretvalue"

	naisResources := []NaisResource{
		{
			resource1Name,
			resource1Type,
			map[string]string{resource1Key: resource1Value},
			map[string]string{secret1Key: secret1Value},
		},
		{
			resource2Name,
			resource2Type,
			map[string]string{resource2Key: resource2Value},
			map[string]string{secret2Key: secret2Value},
		},
		{
			invalidlyNamedResourceNameDot,
			invalidlyNamedResourceTypeDot,
			map[string]string{invalidlyNamedResourceKeyDot: invalidlyNamedResourceValueDot},
			map[string]string{invalidlyNamedResourceSecretKeyDot: invalidlyNamedResourceSecretValueDot},
		},
		{
			invalidlyNamedResourceNameColon,
			invalidlyNamedResourceTypeColon,
			map[string]string{invalidlyNamedResourceKeyColon: invalidlyNamedResourceValueColon},
			map[string]string{invalidlyNamedResourceSecretKeyColon: invalidlyNamedResourceSecretValueColon},
		},
	}

	appConfig := NaisAppConfig{
		Image: image,
		Port:  port,
		Healthcheck: Healthcheck{
			Readiness: Probe{
				Path: readinessPath,
			},
			Liveness: Probe{
				Path: livenessPath,
			},
		},
		Resources: ResourceRequirements{
			Requests: ResourceList{
				Memory: memoryRequest,
				Cpu:    cpuRequest,
			},
			Limits: ResourceList{
				Memory: memoryLimit,
				Cpu:    cpuLimit,
			},
		},
		Prometheus: PrometheusConfig{
			Path:    "/path",
			Enabled: true,
		},
	}

	deployment := createDeploymentDef(naisResources, appConfig, NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: version}, nil)
	deployment.ObjectMeta.ResourceVersion = resourceVersion

	clientset := fake.NewSimpleClientset(deployment)

	t.Run("Nonexistant deployment yields empty string and no error", func(t *testing.T) {
		nilValue, err := getExistingDeployment("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing deployment yields def and no error", func(t *testing.T) {
		id, err := getExistingDeployment(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, id.ObjectMeta.ResourceVersion)
	})

	t.Run("when no deployment exists, it's created", func(t *testing.T) {
		deployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName, Version: version}, appConfig, naisResources, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, deployment.Name)
		assert.Equal(t, "", deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherAppName, deployment.Spec.Template.Name)

		containers := deployment.Spec.Template.Spec.Containers
		container := containers[0]
		assert.Equal(t, otherAppName, container.Name)
		assert.Equal(t, image+":"+version, container.Image)
		assert.Equal(t, int32(port), container.Ports[0].ContainerPort)
		assert.Equal(t, livenessPath, container.LivenessProbe.HTTPGet.Path)
		assert.Equal(t, readinessPath, container.ReadinessProbe.HTTPGet.Path)
		assert.Equal(t, int32(20), deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds)
		assert.Equal(t, int32(20), deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds)

		ptr := func(p resource.Quantity) *resource.Quantity {
			return &p
		}
		assert.Equal(t, memoryRequest, ptr(container.Resources.Requests["memory"]).String())
		assert.Equal(t, memoryLimit, ptr(container.Resources.Limits["memory"]).String())
		assert.Equal(t, cpuRequest, ptr(container.Resources.Requests["cpu"]).String())
		assert.Equal(t, cpuLimit, ptr(container.Resources.Limits["cpu"]).String())
		assert.Equal(t, map[string]string{
			"prometheus.io/scrape":"true",
			"prometheus.io/path":"/path",
			"prometheus.io/port":"http",
		}, deployment.Spec.Template.Annotations)

		env := container.Env
		assert.Equal(t, 9, len(env))
		assert.Equal(t, version, env[0].Value)
		assert.Equal(t, resource1Name+"_"+resource1Key, env[1].Name)
		assert.Equal(t, "value1", env[1].Value)
		assert.Equal(t, resource1Name+"_"+secret1Key, env[2].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret1Key, resource1Name), env[2].ValueFrom)
		assert.Equal(t, resource2Name+"_"+resource2Key, env[3].Name)
		assert.Equal(t, "value2", env[3].Value)
		assert.Equal(t, resource2Name+"_"+secret2Key, env[4].Name)
		assert.Equal(t, createSecretRef(otherAppName, secret2Key, resource2Name), env[4].ValueFrom)
		assert.Equal(t, "dots_are_not_allowed_key", env[5].Name)
		assert.Equal(t, "dots_are_not_allowed_secretkey", env[6].Name)
		assert.Equal(t, "colon_are_not_allowed_key", env[7].Name)
		assert.Equal(t, "colon_are_not_allowed_secretkey", env[8].Name)
	})

	t.Run("when a deployment exists, its updated", func(t *testing.T) {
		updatedDeployment, err := createOrUpdateDeployment(NaisDeploymentRequest{Namespace: namespace, Application: appName, Version: newVersion}, appConfig, naisResources, clientset)
		assert.NoError(t, err)

		assert.Equal(t, resourceVersion, deployment.ObjectMeta.ResourceVersion)
		assert.Equal(t, appName, updatedDeployment.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Name)
		assert.Equal(t, appName, updatedDeployment.Spec.Template.Spec.Containers[0].Name)
		assert.Equal(t, image+":"+newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, int32(port), updatedDeployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, newVersion, updatedDeployment.Spec.Template.Spec.Containers[0].Env[0].Value)
	})
}

func TestIngress(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	subDomain := "example.no"
	ingress := createIngressDef(subDomain, appName, namespace)
	ingress.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(ingress)

	t.Run("Nonexistant ingress yields nil and no error", func(t *testing.T) {
		ingress, err := getExistingIngress("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, ingress)
	})

	t.Run("Existing ingress yields def and no error", func(t *testing.T) {
		existingIngress, err := getExistingIngress(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingIngress.ObjectMeta.ResourceVersion)
	})

	t.Run("when no ingress exists, a new one is created", func(t *testing.T) {
		ingress, err := createIngress(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, subDomain, clientset)

		assert.NoError(t, err)
		assert.Equal(t, otherAppName, ingress.ObjectMeta.Name)
		assert.Equal(t, otherAppName+"."+subDomain, ingress.Spec.Rules[0].Host)
		assert.Equal(t, otherAppName, ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
		assert.Equal(t, intstr.FromInt(80), ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort)
	})

	t.Run("when an ingress exists, nothing happens", func(t *testing.T) {
		nilValue, err := createIngress(NaisDeploymentRequest{Namespace: namespace, Application: appName}, subDomain, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})
}

func TestCreateOrUpdateSecret(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	resource1Name := "r1"
	resource1Type := "db"
	resource1Key := "key1"
	resource1Value := "value1"
	secret1Key := "password"
	secret1Value := "secret"
	resource2Name := "r2"
	resource2Type := "db"
	resource2Key := "key2"
	resource2Value := "value2"
	secret2Key := "password"
	secret2Value := "anothersecret"

	naisResources := []NaisResource{
		{resource1Name, resource1Type, map[string]string{resource1Key: resource1Value}, map[string]string{secret1Key: secret1Value}},
		{resource2Name, resource2Type, map[string]string{resource2Key: resource2Value}, map[string]string{secret2Key: secret2Value}}}

	secret := createSecretDef(naisResources, nil, appName, namespace)
	secret.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(secret)

	t.Run("Nonexistant secret yields nil and no error", func(t *testing.T) {
		nilValue, err := getExistingSecret("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nilValue)
	})

	t.Run("Existing secret yields def and no error", func(t *testing.T) {
		existingSecret, err := getExistingSecret(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingSecret.ObjectMeta.ResourceVersion)
	})

	t.Run("when no secret exists, a new one is created", func(t *testing.T) {
		secret, err := createOrUpdateSecret(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, naisResources, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, otherAppName, secret.ObjectMeta.Name)
		assert.Equal(t, 2, len(secret.Data))
		assert.Equal(t, []byte(secret1Value), secret.Data[resource1Name+"_"+secret1Key])
		assert.Equal(t, []byte(secret2Value), secret.Data[resource2Name+"_"+secret2Key])
	})

	t.Run("when a secret exists, it's updated", func(t *testing.T) {
		updatedSecretValue := "newsecret"
		secret, err := createOrUpdateSecret(NaisDeploymentRequest{Namespace: namespace, Application: appName}, []NaisResource{
			{resource1Name, resource1Type, nil, map[string]string{secret1Key: updatedSecretValue}}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, secret.ObjectMeta.ResourceVersion)
		assert.Equal(t, namespace, secret.ObjectMeta.Namespace)
		assert.Equal(t, appName, secret.ObjectMeta.Name)
		assert.Equal(t, []byte(updatedSecretValue), secret.Data[resource1Name+"_"+secret1Key])
	})
}

func TestCreateOrUpdateAutoscaler(t *testing.T) {
	autoscaler := createOrUpdateAutoscalerDef(1, 2, 3, nil, appName, namespace)
	autoscaler.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(autoscaler)

	t.Run("nonexistant autoscaler yields empty string and no error", func(t *testing.T) {
		nonExistingAutoscaler, err := getExistingAutoscaler("nonexisting", namespace, clientset)
		assert.NoError(t, err)
		assert.Nil(t, nonExistingAutoscaler)
	})

	t.Run("existing autoscaler yields id and no error", func(t *testing.T) {
		existingAutoscaler, err := getExistingAutoscaler(appName, namespace, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, existingAutoscaler.ObjectMeta.ResourceVersion)
	})

	t.Run("when no autoscaler exists, a new one is created", func(t *testing.T) {
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: namespace, Application: otherAppName}, NaisAppConfig{Replicas: Replicas{Max: 1, Min: 2, CpuThresholdPercentage: 69}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, "", autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, int32(1), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, int32p(2), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32p(69), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, namespace, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, otherAppName, autoscaler.ObjectMeta.Name)
		assert.Equal(t, otherAppName, autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})

	t.Run("when autoscaler exists, it's updated", func(t *testing.T) {
		cpuThreshold := 69
		minReplicas := 6
		maxReplicas := 9
		autoscaler, err := createOrUpdateAutoscaler(NaisDeploymentRequest{Namespace: namespace, Application: appName}, NaisAppConfig{Replicas: Replicas{CpuThresholdPercentage: cpuThreshold, Min: minReplicas, Max: maxReplicas}}, clientset)
		assert.NoError(t, err)
		assert.Equal(t, resourceVersion, autoscaler.ObjectMeta.ResourceVersion)
		assert.Equal(t, namespace, autoscaler.ObjectMeta.Namespace)
		assert.Equal(t, appName, autoscaler.ObjectMeta.Name)
		assert.Equal(t, int32p(int32(cpuThreshold)), autoscaler.Spec.TargetCPUUtilizationPercentage)
		assert.Equal(t, int32p(int32(minReplicas)), autoscaler.Spec.MinReplicas)
		assert.Equal(t, int32(maxReplicas), autoscaler.Spec.MaxReplicas)
		assert.Equal(t, appName, autoscaler.Spec.ScaleTargetRef.Name)
		assert.Equal(t, "Deployment", autoscaler.Spec.ScaleTargetRef.Kind)
	})
}

func TestCreateK8sResources(t *testing.T) {
	deploymentRequest := NaisDeploymentRequest{
		Application:  appName,
		Version:      version,
		Environment:  namespace,
		AppConfigUrl: "http://repo.com/app",
		Zone:         "zone",
		Namespace:    namespace,
	}

	appConfig := NaisAppConfig{
		Image: image,
		Port:  port,
		Resources: ResourceRequirements{
			Requests: ResourceList{
				Cpu:    cpuRequest,
				Memory: memoryRequest,
			},
			Limits: ResourceList{
				Cpu:    cpuLimit,
				Memory: memoryLimit,
			},
		},
	}

	naisResources := []NaisResource{
		{"resourceName", "resourceType", map[string]string{"resourceKey": "resource1Value"}, map[string]string{"secretKey": "secretValue"}}}

	service := createOrUpdateServiceDef(69, nil, appName, namespace)
	service.ObjectMeta.ResourceVersion = resourceVersion
	clientset := fake.NewSimpleClientset(service)

	t.Run("creates all resources", func(t *testing.T) {
		deploymentResult, error := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResources, "nais.example.yo", clientset)
		assert.NoError(t, error)

		assert.NotEmpty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
		assert.NotEmpty(t, deploymentResult.Deployment)
		assert.NotEmpty(t, deploymentResult.Ingress)
		assert.NotEmpty(t, deploymentResult.Autoscaler)

		assert.Equal(t, resourceVersion, deploymentResult.Service.ObjectMeta.ResourceVersion, "service should have same id as the preexisting")
		assert.Equal(t, "", deploymentResult.Secret.ObjectMeta.ResourceVersion, "secret should not have any id set")
	})

	naisResourcesNoSecret := []NaisResource{
		{"resourceName", "resourceType", map[string]string{"resourceKey": "resource1Value"}, map[string]string{}}}

	t.Run("omits secret creation when no secret resources ex", func(t *testing.T) {
		deploymentResult, error := createOrUpdateK8sResources(deploymentRequest, appConfig, naisResourcesNoSecret, "nais.example.yo", fake.NewSimpleClientset())
		assert.NoError(t, error)

		assert.Empty(t, deploymentResult.Secret)
		assert.NotEmpty(t, deploymentResult.Service)
	})

}

func createSecretRef(appName string, resKey string, resName string) *v1.EnvVarSource {
	return &v1.EnvVarSource{
		SecretKeyRef: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: appName,
			},
			Key: resName + "_" + resKey,
		},
	}
}
