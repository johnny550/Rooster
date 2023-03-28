package tests

import (
	"context"
	"fmt"
	"testing"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StreamlinerUtilsTest struct {
	suite.Suite
}

const (
	ns = "default"
)

var (
	customDeleteOptions = new(meta_v1.DeleteOptions)
)

type plural struct {
	Group    string
	Version  string
	Resource string
}

func (suite *StreamlinerUtilsTest) SetupSuite() {
	fmt.Println(" SetupSuite")
	customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	fmt.Printf("customDeleteOptions: %v\n", customDeleteOptions)
}

func (suite *StreamlinerUtilsTest) TestGroupVersionGuess() {
	apiVersion := "v1"
	kind := "pod"
	expectedResult := plural{}
	expectedResult.Group = ""
	expectedResult.Resource = kind + "s"
	expectedResult.Version = apiVersion
	val, err := utils.UnsafeGuessGroupVersionResource(apiVersion, kind)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedResult.Group, val.Group)
	assert.Equal(suite.T(), expectedResult.Version, val.Version)
	assert.Equal(suite.T(), expectedResult.Resource, val.Resource)
}

func (suite *StreamlinerUtilsTest) TestShellScript() {
	cmd := "pwd"
	result, err := utils.Shell(cmd)
	assert.Nil(suite.T(), err)
	assert.NotEmpty(suite.T(), result)
	assert.Contains(suite.T(), result, "Rooster/pkg/tests")
}

func (suite *StreamlinerUtilsTest) TestDeleteService() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	// Get services
	svcs, err := m.GetClient().CoreV1().Services(ns).List(context.TODO(), meta_v1.ListOptions{})
	assert.Nil(suite.T(), err)
	if len(svcs.Items) == 0 {
		fmt.Println("No service found")
		return
	}
	targetSvc := svcs.Items[0].Name
	done, err := utils.DeleteService(*m, ns, targetSvc, *customDeleteOptions)
	assert.True(suite.T(), done)
	assert.Nil(suite.T(), err)
}

func (suite *StreamlinerUtilsTest) TestDeleteServiceAccount() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	// Get service accounts
	sas, err := m.GetClient().CoreV1().ServiceAccounts(ns).List(context.TODO(), meta_v1.ListOptions{})
	assert.Nil(suite.T(), err)
	if len(sas.Items) == 0 {
		fmt.Println("No service account found")
		return
	}
	targetSa := sas.Items[0].Name
	done, err := utils.DeleteServiceAccount(*m, ns, targetSa, *customDeleteOptions)
	assert.True(suite.T(), done)
	assert.Nil(suite.T(), err)
}

func (suite *StreamlinerUtilsTest) TestDeleteConfigMap() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	// Get config maps
	cms, err := m.GetClient().CoreV1().ConfigMaps(ns).List(context.TODO(), meta_v1.ListOptions{})
	assert.Nil(suite.T(), err)
	if len(cms.Items) == 0 {
		fmt.Println("No config map found")
		return
	}
	targetCM := cms.Items[0].Name
	done, err := utils.DeleteConfigMap(*m, ns, targetCM, *customDeleteOptions)
	assert.True(suite.T(), done)
	assert.Nil(suite.T(), err)
}

func (suite *StreamlinerUtilsTest) TestDeleteDaemonSet() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	// Get daemon sets
	daemonSets, err := m.GetClient().AppsV1().DaemonSets(ns).List(context.TODO(), meta_v1.ListOptions{})
	assert.Nil(suite.T(), err)
	if len(daemonSets.Items) == 0 {
		fmt.Println("No daemonsets found")
		return
	}
	targetDs := daemonSets.Items[0].Name
	done, err := utils.DeleteDaemonSet(*m, ns, targetDs, *customDeleteOptions)
	assert.True(suite.T(), done)
	assert.Nil(suite.T(), err)
}

func (suite *StreamlinerUtilsTest) TestCreate() {
	customCreateOptions := meta_v1.CreateOptions{}
	customCreateOptions.DryRun = append(customCreateOptions.DryRun, "All")
}

func TestUtils(t *testing.T) {
	s := new(StreamlinerUtilsTest)
	suite.Run(t, s)
}
