package main_test

import (
	"encoding/json"
	"errors"
	"os/exec"

	. "github.com/thomaseizinger/cf-get-env-plugin"

	"code.cloudfoundry.org/cli/testhelpers/rpcserver"
	"code.cloudfoundry.org/cli/testhelpers/rpcserver/rpcserverfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"code.cloudfoundry.org/cli/plugin/models"
)

const validPluginPath = "./test_rpc_server_example.exe"

var _ = Describe("App-Lister", func() {

	var (
		rpcHandlers *rpcserverfakes.FakeHandlers
		ts          *rpcserver.TestServer
		err         error
	)

	BeforeEach(func() {
		rpcHandlers = new(rpcserverfakes.FakeHandlers)
		ts, err = rpcserver.NewTestRPCServer(rpcHandlers)
		Expect(err).NotTo(HaveOccurred())

		//set rpc.CallCoreCommand to a successful call
		//rpc.CallCoreCommand is used in both cliConnection.CliCommand() and
		//cliConnection.CliWithoutTerminalOutput()
		rpcHandlers.CallCoreCommandStub = func(_ []string, retVal *bool) error {
			*retVal = true
			return nil
		}

		//set rpc.GetOutputAndReset to return empty string; this is used by CliCommand()/CliWithoutTerminalOutput()
		rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
			*retVal = []string{"{}"}
			return nil
		}
	})

	JustBeforeEach(func() {
		err = ts.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ts.Stop()
	})

	Describe("get-env", func() {
		Context("Running the command", func() {
			Context("Getting app endpoint", func() {
				BeforeEach(func() {
					rpcHandlers.GetAppsStub = func(_ string, retVal *plugin_models.GetAppModel) error {
						retVal = plugin_models.GetAppModel{
							Guid:"1234",
						}
						return nil
					}

					rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
						*retVal = []string{marshal(sampleApps())}
						return nil
					}
				})

				It("shows the endpoint it is curling", func() {
					args := []string{ts.Port(), "list-apps"}
					session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
					session.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(session).To(gbytes.Say("api.example.com/v2/apps"))
				})

				Context("when ApiEndpoint() returns an error", func() {
					BeforeEach(func() {
						rpcHandlers.ApiEndpointStub = func(_ string, retVal *string) error {
							*retVal = ""
							return errors.New("Bad bad error")
						}
					})

					It("raises an error when ApiEndpoint() returns an error", func() {
						args := []string{ts.Port(), "list-apps"}
						session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
						session.Wait()
						Expect(err).NotTo(HaveOccurred())
						Expect(session).To(gbytes.Say("FAILED"))
						Expect(session).To(gbytes.Say("Bad bad error"))
						Expect(session.ExitCode()).To(Equal(1))
					})
				})

				Context("when getting a list of apps", func() {
					Context("without option flag", func() {
						BeforeEach(func() {
							rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
								*retVal = []string{marshal(sampleApps())}
								return nil
							}
						})

						It("lists all apps", func() {
							args := []string{ts.Port(), "list-apps"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(session).To(gbytes.Say("app1"))
							Expect(session).To(gbytes.Say("app2"))
							Expect(session).To(gbytes.Say("app3"))
						})
					})

					Context("with --started", func() {
						BeforeEach(func() {
							rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
								*retVal = []string{marshal(sampleApps())}
								return nil
							}
						})

						It("lists only started apps", func() {
							args := []string{ts.Port(), "list-apps", "--started"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(session).To(gbytes.Say("app1"))
							Expect(session).To(gbytes.Say("app2"))
							Expect(session).NotTo(gbytes.Say("app3"))
						})
					})

					Context("with --stopped", func() {
						BeforeEach(func() {
							rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
								*retVal = []string{marshal(sampleApps())}
								return nil
							}
						})

						It("lists only stopped apps", func() {
							args := []string{ts.Port(), "list-apps", "--stopped"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(session).NotTo(gbytes.Say("app1"))
							Expect(session).NotTo(gbytes.Say("app2"))
							Expect(session).To(gbytes.Say("app3"))
						})
					})

					Context("when CliCommandWithoutTerminalOutput() returns an error", func() {
						BeforeEach(func() {
							rpcHandlers.CallCoreCommandStub = func(_ []string, retVal *bool) error {
								return errors.New("something went wrong")
							}
						})

						It("notifies the user about the error", func() {
							args := []string{ts.Port(), "list-apps", "--stopped"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(session).To(gbytes.Say("FAILED"))
							Expect(session).To(gbytes.Say("something went wrong"))
						})
					})

					Context("when 'next url' is present in the JSON response", func() {
						BeforeEach(func() {
							count := 0
							rpcHandlers.GetOutputAndResetStub = func(_ bool, retVal *[]string) error {
								apps := sampleApps()
								if count == 0 {
									apps.NextURL = "v2/apps?page=2"
									*retVal = []string{marshal(apps)}
									count++
								} else {
									apps.Resources = append(apps.Resources, AppModel{Entity: EntityModel{Name: "app4", State: "STARTED"}})
									*retVal = []string{marshal(apps)}
								}
								return nil
							}
						})

						It("follows and curl the next url", func() {
							args := []string{ts.Port(), "list-apps"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(rpcHandlers.CallCoreCommandCallCount()).To(Equal(2))

							params, _ := rpcHandlers.CallCoreCommandArgsForCall(0)
							Expect(params[1]).To(Equal("v2/apps"))

							params, _ = rpcHandlers.CallCoreCommandArgsForCall(1)
							Expect(params[1]).To(Equal("v2/apps?page=2"))
						})

						It("traverses through all pages and list all the apps", func() {
							args := []string{ts.Port(), "list-apps"}
							session, err := gexec.Start(exec.Command(validPluginPath, args...), GinkgoWriter, GinkgoWriter)
							session.Wait()
							Expect(err).NotTo(HaveOccurred())
							Expect(session).To(gbytes.Say("app1"))
							Expect(session).To(gbytes.Say("app2"))
							Expect(session).To(gbytes.Say("app3"))
							Expect(session).To(gbytes.Say("app4"))
						})
					})
				})
			})
		})
	})
})

func sampleApps() AppsModel {
	allApps := AppsModel{
		Resources: []AppModel{
			{
				EntityModel{Name: "app1", State: "STARTED"},
			},
			{
				EntityModel{Name: "app2", State: "STARTED"},
			},
			{
				EntityModel{Name: "app3", State: "STOPPED"},
			},
		},
	}

	return allApps
}

func marshal(apps AppsModel) string {
	b, err := json.Marshal(apps)
	Expect(err).NotTo(HaveOccurred())

	return string(b)
}
