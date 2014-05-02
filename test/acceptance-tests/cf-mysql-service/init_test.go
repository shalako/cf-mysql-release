package cf_mysql_service

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"../helpers"
	. "github.com/pivotal-cf-experimental/cf-test-helpers/runner"
)

func TestServices(t *testing.T) {
	helpers.SetupEnvironment(helpers.NewContext(IntegrationConfig))
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "P-MySQL Acceptance Tests", []Reporter{reporters.NewJUnitReporter(fmt.Sprintf("junit_%d.xml", ginkgoconfig.GinkgoConfig.ParallelNode))})
}

func AppUri(appname string) string {
	return "http://" + appname + "." + IntegrationConfig.AppsDomain
}

func SystemUri(componentName string) string {
	return "http://" + componentName + "." + IntegrationConfig.SystemDomain
}

func Curling(args ...string) func() *gexec.Session {
	return func() *gexec.Session {
		return Curl(args...)
	}
}

var IntegrationConfig = helpers.LoadConfig()
var sinatraPath = "../assets/sinatra_app"