package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"goji.io"
	"goji.io/pat"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeDeployStatusViewer struct {
	deployStatusToReturn DeployStatus
	viewToReturn         DeploymentStatusView
	errToReturn          error
}

func (d FakeDeployStatusViewer) DeploymentStatusView(namespace string, deployName string) (DeployStatus, DeploymentStatusView, error) {
	return d.deployStatusToReturn, d.viewToReturn, d.errToReturn
}

func TestAnIncorrectPayloadGivesError(t *testing.T) {
	api := Api{}

	body := strings.NewReader("gibberish")

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
}

func TestDeployStatusHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/deploystatus/namespace/deployName", strings.NewReader("whatever"))

	t.Run("Return 404 if deploy status is not found", func(t *testing.T) {
		mux := goji.NewMux()

		api := Api{DeploymentStatusViewer: FakeDeployStatusViewer{
			errToReturn: fmt.Errorf("not Found"),
		}}

		mux.Handle(pat.Get("/deploystatus/:namespace/:deployName"), appHandler(api.deploymentStatusHandler))

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Correct http code for a given deploy status", func(t *testing.T) {

		tests := []struct {
			deployStatus     DeployStatus
			expectedHttpCode int
		}{
			{
				Success,
				200,
			},
			{
				Failed,
				500,
			},
			{
				InProgress,
				202,
			},
		}

		for _, test := range tests {
			mux := goji.NewMux()

			api := Api{
				DeploymentStatusViewer: FakeDeployStatusViewer{
					deployStatusToReturn: test.deployStatus,
				},
			}
			mux.Handle(pat.Get("/deploystatus/:namespace/:deployName"), appHandler(api.deploymentStatusHandler))

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			assert.Equal(t, test.expectedHttpCode, rr.Code)
		}
	})
}

func TestNoManifestGivesError(t *testing.T) {
	api := Api{}

	manifestUrl := "http://repo.com/app"
	depReq := NaisDeploymentRequest{
		Application:      "appname",
		Version:          "",
		FasitEnvironment: "",
		ManifestUrl:      manifestUrl,
		Zone:             "zone",
		Namespace:        "namespace",
	}

	defer gock.Off()

	gock.New("http://repo.com").
		Get("/app").
		Reply(400).
		JSON(map[string]string{"foo": "bar"})

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, err := http.NewRequest("POST", "/deploy", body)

	if err != nil {
		panic("could not create req")
	}
	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 500, rr.Code)
	assert.Contains(t, string(rr.Body.Bytes()), manifestUrl)
}

//TODO remove once grace period ends
func TestWarningsWhenUsingOldPropertyNames(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	environment := "environmentName"
	image := "name/Container"
	version := "123"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk", "test-cluster", false, nil}

	depReq := NaisDeploymentRequest{
		Application: appName,
		Version:     version,
		Environment: environment,
		Username:    "user",
		Password:    "password",
		ManifestUrl: "http://repo.com/app",
		Zone:        "zone",
		Namespace:   namespace,
	}

	manifest := NaisManifest{
		Image: image,
		Port:  321,
	}
	data, _ := yaml.Marshal(manifest)

	defer gock.Off()

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitTruststoreResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).
		BodyString("")

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created secret\n- created service\n- created ingress\n- created autoscaler\n\nWarnings:\n- Deployment request property 'environment' is deprecated. Use 'fasitEnvironment' instead\n- Deployment request property 'username' is deprecated. Use 'fasitUsername' instead\n- Deployment request property 'password' is deprecated. Use 'fasitPassword' instead\n", string(rr.Body.Bytes()))
}
func TestValidDeploymentRequestAndManifestCreateResources(t *testing.T) {
	appName := "appname"
	namespace := "namespace"
	environment := "environmentName"
	image := "name/Container"
	version := "123"
	resourceAlias := "alias1"
	resourceType := "db"
	zone := "zone"

	clientset := fake.NewSimpleClientset()

	api := Api{clientset, "https://fasit.local", "nais.example.tk", "test-cluster", false, nil}

	depReq := NaisDeploymentRequest{
		Application:      appName,
		Version:          version,
		FasitEnvironment: environment,
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Namespace:        namespace,
	}

	manifest := NaisManifest{
		Image: image,
		Port:  321,
		FasitResources: FasitResources{
			Used: []UsedResource{{resourceAlias, resourceType, nil}},
		},
	}
	response := "anything"
	data, _ := yaml.Marshal(manifest)
	appInstanceResponse, _ := yaml.Marshal(response)

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitTruststoreResponse.json")

	gock.New("https://fasit.local").
		Get("/api/v2/resources/3024713/file/keystore").
		Reply(200).
		BodyString("")

	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))

	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", resourceAlias).
		MatchParam("type", resourceType).
		MatchParam("environment", environment).
		MatchParam("application", appName).
		MatchParam("zone", zone).
		Reply(200).File("testdata/fasitResponse.json")

	gock.New("https://fasit.local").
		Get(fmt.Sprintf("/api/v2/environments/%s", environment)).
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})

	gock.New("https://fasit.local").
		Get("/api/v2/applications/" + appName).
		Reply(200).
		BodyString("anything")

	gock.New("https://fasit.local").
		Post("/api/v2/applicationinstances/").
		Reply(200).
		BodyString(string(appInstanceResponse))

	jsn, _ := json.Marshal(depReq)

	body := strings.NewReader(string(jsn))

	req, _ := http.NewRequest("POST", "/deploy", body)

	rr := httptest.NewRecorder()
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.True(t, gock.IsDone())
	assert.Equal(t, "result: \n- created deployment\n- created secret\n- created service\n- created ingress\n- created autoscaler\n", string(rr.Body.Bytes()))
}

func TestMissingResources(t *testing.T) {
	resourceAlias := "alias1"
	resourceType := "db"

	manifest := NaisManifest{
		Image: "name/Container",
		Port:  321,
		FasitResources: FasitResources{
			Used: []UsedResource{{resourceAlias, resourceType, nil}},
		},
	}
	data, _ := yaml.Marshal(manifest)

	defer gock.Off()
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		MatchParam("alias", NavTruststoreFasitAlias).
		Reply(200).File("testdata/fasitResponse.json")
	gock.New("http://repo.com").
		Get("/app").
		Reply(200).
		BodyString(string(data))
	gock.New("https://fasit.local").
		Get("/api/v2/environments/namespace").
		Reply(200).
		JSON(map[string]string{"environmentclass": "u"})
	gock.New("https://fasit.local").
		Get("/api/v2/applications/appname").
		Reply(200).
		BodyString("anything")
	gock.New("https://fasit.local").
		Get("/api/v2/scopedresource").
		Reply(404)

	req, _ := http.NewRequest("POST", "/deploy", strings.NewReader(CreateDefaultDeploymentRequest()))

	rr := httptest.NewRecorder()
	api := Api{fake.NewSimpleClientset(), "https://fasit.local", "nais.example.tk", "clustername", false, nil}
	handler := http.Handler(appHandler(api.deploy))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 400, rr.Code)
	assert.True(t, gock.IsDone())

	assert.Contains(t, string(rr.Body.Bytes()), fmt.Sprintf("unable to get resource %s (%s)", resourceAlias, resourceType))
}

func CreateDefaultDeploymentRequest() string {
	jsn, _ := json.Marshal(NaisDeploymentRequest{
		Application:      "appname",
		Version:          "123",
		FasitEnvironment: "namespace",
		ManifestUrl:      "http://repo.com/app",
		Zone:             "zone",
		Namespace:        "namespace",
	})

	return string(jsn)
}

func TestValidateDeploymentRequest(t *testing.T) {
	t.Run("Empty fields should be marked invalid", func(t *testing.T) {
		invalid := NaisDeploymentRequest{
			Application:      "",
			Version:          "",
			FasitEnvironment: "",
			Zone:             "",
			Namespace:        "",
			Username:         "",
			Password:         "",
		}

		err := invalid.Validate()

		assert.NotNil(t, err)
		assert.Contains(t, err, errors.New("application is required and is empty"))
		assert.Contains(t, err, errors.New("version is required and is empty"))
		assert.Contains(t, err, errors.New("environment is required and is empty"))
		assert.Contains(t, err, errors.New("zone is required and is empty"))
		assert.Contains(t, err, errors.New("zone can only be fss, sbs or iapp"))
		assert.Contains(t, err, errors.New("namespace is required and is empty"))
		assert.Contains(t, err, errors.New("username is required and is empty"))
		assert.Contains(t, err, errors.New("password is required and is empty"))
	})
}
