package cf_mysql_service

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"

	"fmt"
	"strconv"
	"time"

	"../helpers"

	. "github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	. "github.com/cloudfoundry-incubator/cf-test-helpers/runner"
)

var (
	_ = Describe("P-MySQL Service", func() {
		timeout := 120 * time.Second
		retryInterval := 1.0

		AssertAppIsRunning := func(appName string) {
			pingUri := AppUri(appName) + "/ping"
			fmt.Println("Checking that the app is responding at url: ", pingUri)
			Eventually(Curl(pingUri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("OK"))
			fmt.Println("\n")
		}

		It("Registers a route", func() {
			uri := "http://" + IntegrationConfig.BrokerHost + "/v2/catalog"

			fmt.Println("Curling url: ", uri)
			Eventually(Curl(uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("HTTP Basic: Access denied."))
		})

		Describe("Service instance lifecycle", func() {
			var appName string

			BeforeEach(func() {
				appName = RandomName()
				Eventually(Cf("push", appName, "-m", "256M", "-p", sinatraPath, "-no-start"), helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))
			})

			AfterEach(func() {
				Eventually(Cf("delete", appName, "-f"), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
			})

			AssertLifeCycleBehavior := func(PlanName string) {
				It("Allows users to create, bind, write to, read from, unbind, and destroy a service instance a plan", func() {
					serviceInstanceName := RandomName()
					uri := AppUri(appName) + "/service/mysql/" + serviceInstanceName + "/mykey"

					Eventually(Cf("create-service", IntegrationConfig.ServiceName, PlanName, serviceInstanceName),
						helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))

					Eventually(Cf("bind-service", appName, serviceInstanceName), helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))
					Eventually(Cf("start", appName), helpers.ScaledTimeout(5*time.Minute)).Should(Exit(0))
					AssertAppIsRunning(appName)

					fmt.Println("Posting to url: ", uri)
					Eventually(Curl("-d", "myvalue", uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("myvalue"))
					fmt.Println("\n")

					fmt.Println("Curling url: ", uri)
					Eventually(Curl(uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("myvalue"))
					fmt.Println("\n")

					Eventually(Cf("unbind-service", appName, serviceInstanceName), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
					Eventually(Cf("delete-service", "-f", serviceInstanceName), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
				})
			}

			Context("using a new service instance", func() {
				for _, plan := range IntegrationConfig.Plans {
					AssertLifeCycleBehavior(plan.Name)
				}
			})
		})

		Describe("Enforcing MySQL storage and connection quota", func() {
			var appName string
			var serviceInstanceName string

			BeforeEach(func() {
				appName = RandomName()
				serviceInstanceName = RandomName()

				Eventually(Cf("push", appName, "-m", "256M", "-p", sinatraPath, "-no-start"), helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))
			})

			AfterEach(func() {
				Eventually(Cf("unbind-service", appName, serviceInstanceName), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
				Eventually(Cf("delete-service", "-f", serviceInstanceName), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
				Eventually(Cf("delete", appName, "-f"), helpers.ScaledTimeout(20*time.Second)).Should(Exit(0))
			})

			CreatesBindsAndStartsApp := func(PlanName string) {
				Eventually(Cf("create-service", IntegrationConfig.ServiceName, PlanName, serviceInstanceName),
					helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))
				Eventually(Cf("bind-service", appName, serviceInstanceName), helpers.ScaledTimeout(60*time.Second)).Should(Exit(0))
				Eventually(Cf("start", appName), helpers.ScaledTimeout(5*time.Minute)).Should(Exit(0))
				AssertAppIsRunning(appName)
			}

			AssertStorageQuotaBehavior := func(PlanName string, MaxStorageMb int) {
				It("enforces the storage quota for the plan", func() {
					CreatesBindsAndStartsApp(PlanName)

					quotaEnforcerSleepTime := 10 * time.Second
					uri := AppUri(appName) + "/service/mysql/" + serviceInstanceName + "/mykey"
					writeUri := AppUri(appName) + "/service/mysql/" + serviceInstanceName + "/write-bulk-data"
					deleteUri := AppUri(appName) + "/service/mysql/" + serviceInstanceName + "/delete-bulk-data"
					firstValue := RandomName()[:20]
					secondValue := RandomName()[:20]

					fmt.Println("*** Proving we can write")
					Eventually(Curl("-d", firstValue, uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say(firstValue))
					fmt.Println("*** Proving we can read")
					Eventually(Curl(uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say(firstValue))

					fmt.Println("*** Exceeding quota")

					mbToWrite := 10
					loopIterations := (MaxStorageMb / mbToWrite)
					if MaxStorageMb%mbToWrite == 0 {
						loopIterations += 1
					}

					for i := 0; i < loopIterations; i += 1 {
						Eventually(Curl("-v", "-d", strconv.Itoa(mbToWrite), writeUri), helpers.ScaledTimeout(5*time.Minute), retryInterval).Should(Say("Database now contains"))
					}

					fmt.Println("*** Sleeping to let quota enforcer run")
					time.Sleep(quotaEnforcerSleepTime)

					fmt.Println("*** Proving we cannot write")
					Eventually(Curl("-d", firstValue, uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("Error: (INSERT|UPDATE) command denied .* for table 'data_values'"))
					fmt.Println("*** Proving we can read")
					Eventually(Curl(uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say(firstValue))

					fmt.Println("*** Deleting below quota")
					Eventually(Curl("-d", "20", deleteUri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("Database now contains"))

					fmt.Println("*** Sleeping to let quota enforcer run")
					time.Sleep(quotaEnforcerSleepTime)

					fmt.Println("*** Proving we can write")
					Eventually(Curl("-d", secondValue, uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say(secondValue))
					fmt.Println("*** Proving we can read")
					Eventually(Curl(uri), helpers.ScaledTimeout(timeout), retryInterval).Should(Say(secondValue))
				})
			}

			AssertConnectionQuotaBehavior := func(PlanName string, MaxUserConnections int) {
				It("enforces the connection quota for the plan", func() {
					CreatesBindsAndStartsApp(PlanName)

					uri := AppUri(appName) + "/connections/mysql/" + serviceInstanceName + "/"
					over_maximum_connection_num := MaxUserConnections + 1

					fmt.Println("*** Proving we can use the max num of connections")

					Eventually(Curl(uri+strconv.Itoa(MaxUserConnections)), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("success"))

					fmt.Println("*** Proving the connection quota is enforced")
					Eventually(Curl(uri+strconv.Itoa(over_maximum_connection_num)), helpers.ScaledTimeout(timeout), retryInterval).Should(Say("Error"))
				})
			}

			Context("for each plan", func() {
				for _, plan := range IntegrationConfig.Plans {
					AssertStorageQuotaBehavior(plan.Name, plan.MaxStorageMb)
					AssertConnectionQuotaBehavior(plan.Name, plan.MaxUserConnections)
				}
			})
		})
	})
)
