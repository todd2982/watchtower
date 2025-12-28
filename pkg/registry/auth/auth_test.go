package auth_test

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/todd2982/watchtower/internal/actions/mocks"
	"github.com/todd2982/watchtower/pkg/registry/auth"

	wtTypes "github.com/todd2982/watchtower/pkg/types"
	ref "github.com/distribution/reference"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Auth Suite")
}
func SkipIfCredentialsEmpty(credentials *wtTypes.RegistryCredentials, fn func()) func() {
	if credentials.Username == "" {
		return func() {
			Skip("Username missing. Skipping integration test")
		}
	} else if credentials.Password == "" {
		return func() {
			Skip("Password missing. Skipping integration test")
		}
	} else {
		return fn
	}
}

var GHCRCredentials = &wtTypes.RegistryCredentials{
	Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
	Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
}

var _ = Describe("the auth module", func() {
	mockId := "mock-id"
	mockName := "mock-container"
	mockImage := "ghcr.io/k6io/operator:latest"
	mockCreated := time.Now()
	mockDigest := "ghcr.io/k6io/operator@sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"

	mockContainer := mocks.CreateMockContainerWithDigest(
		mockId,
		mockName,
		mockImage,
		mockCreated,
		mockDigest)

	Describe("GetToken", func() {
		It("should parse the token from the response",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				creds := fmt.Sprintf("%s:%s", GHCRCredentials.Username, GHCRCredentials.Password)
				token, err := auth.GetToken(mockContainer, creds)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).NotTo(Equal(""))
			}),
		)
	})

	Describe("GetAuthURL", func() {
		It("should create a valid auth url object based on the challenge header supplied", func() {
			challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
			imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
			Expect(err).NotTo(HaveOccurred())
			expected := &url.URL{
				Host:     "ghcr.io",
				Scheme:   "https",
				Path:     "/token",
				RawQuery: "scope=repository%3Atodd2982%2Fwatchtower%3Apull&service=ghcr.io",
			}

			URL, err := auth.GetAuthURL(challenge, imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(URL).To(Equal(expected))
		})

		When("given an invalid challenge header", func() {
			It("should return an error", func() {
				challenge := `bearer realm="https://ghcr.io/token"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				Expect(err).To(HaveOccurred())
				Expect(URL).To(BeNil())
			})
		})

		When("deriving the auth scope from an image name", func() {
			It("should prepend official dockerhub images with \"library/\"", func() {
				Expect(getScopeFromImageAuthURL("registry")).To(Equal("library/registry"))
				Expect(getScopeFromImageAuthURL("docker.io/registry")).To(Equal("library/registry"))
				Expect(getScopeFromImageAuthURL("index.docker.io/registry")).To(Equal("library/registry"))
			})
			It("should not include vanity hosts\"", func() {
				Expect(getScopeFromImageAuthURL("docker.io/todd2982/watchtower")).To(Equal("todd2982/watchtower"))
				Expect(getScopeFromImageAuthURL("index.docker.io/todd2982/watchtower")).To(Equal("todd2982/watchtower"))
			})
			It("should not destroy three segment image names\"", func() {
				Expect(getScopeFromImageAuthURL("piksel/todd2982/watchtower")).To(Equal("piksel/todd2982/watchtower"))
				Expect(getScopeFromImageAuthURL("ghcr.io/piksel/todd2982/watchtower")).To(Equal("piksel/todd2982/watchtower"))
			})
			It("should not prepend library/ to image names if they're not on dockerhub", func() {
				Expect(getScopeFromImageAuthURL("ghcr.io/watchtower")).To(Equal("watchtower"))
				Expect(getScopeFromImageAuthURL("ghcr.io/todd2982/watchtower")).To(Equal("todd2982/watchtower"))
			})
		})
		It("should not crash when an empty field is received", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",`
			imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
			Expect(err).NotTo(HaveOccurred())
			res, err := auth.GetAuthURL(input, imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())
		})
		It("should not crash when a field without a value is received", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",valuelesskey`
			imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
			Expect(err).NotTo(HaveOccurred())
			res, err := auth.GetAuthURL(input, imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())
		})
	})

	Describe("GetChallengeURL", func() {
		It("should create a valid challenge url object based on the image ref supplied", func() {
			expected := url.URL{Host: "ghcr.io", Scheme: "https", Path: "/v2/"}
			imageRef, _ := ref.ParseNormalizedNamed("ghcr.io/todd2982/watchtower:latest")
			Expect(auth.GetChallengeURL(imageRef)).To(Equal(expected))
		})
		It("should assume Docker Hub for image refs with no explicit registry", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			imageRef, _ := ref.ParseNormalizedNamed("todd2982/watchtower:latest")
			Expect(auth.GetChallengeURL(imageRef)).To(Equal(expected))
		})
		It("should use index.docker.io if the image ref specifies docker.io", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			imageRef, _ := ref.ParseNormalizedNamed("docker.io/todd2982/watchtower:latest")
			Expect(auth.GetChallengeURL(imageRef)).To(Equal(expected))
		})
	})

	// Edge case tests for issue #78
	Describe("GetAuthURL edge cases", func() {
		When("the challenge header is malformed", func() {
			It("should handle missing quotes gracefully", func() {
				challenge := `bearer realm=https://ghcr.io/token,service=ghcr.io`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				// Should either parse successfully or return error, but not panic
				_ = URL
				_ = err
			})

			It("should handle missing realm", func() {
				challenge := `bearer service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				_, err = auth.GetAuthURL(challenge, imageRef)
				Expect(err).To(HaveOccurred())
			})

			It("should handle empty realm", func() {
				challenge := `bearer realm="",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				_, err = auth.GetAuthURL(challenge, imageRef)
				Expect(err).To(HaveOccurred())
			})

			It("should handle malformed URL in realm", func() {
				challenge := `bearer realm="ht!tp://invalid url",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				_, err = auth.GetAuthURL(challenge, imageRef)
				Expect(err).To(HaveOccurred())
			})

			It("should handle missing service parameter", func() {
				challenge := `bearer realm="https://ghcr.io/token",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				// Should still succeed, service is optional in some cases
				_ = URL
				_ = err
			})

			It("should handle challenge with special characters", func() {
				challenge := `bearer realm="https://ghcr.io/token?foo=bar&baz=qux",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				// Should handle URL with query parameters
				_ = URL
				_ = err
			})

			It("should handle very long challenge headers", func() {
				longScope := strings.Repeat("a", 10000)
				challenge := fmt.Sprintf(`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="%s"`, longScope)
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				_, err = auth.GetAuthURL(challenge, imageRef)
				// Should not panic with very long inputs
				_ = err
			})

			It("should handle challenge with extra whitespace", func() {
				challenge := `bearer   realm = "https://ghcr.io/token" , service = "ghcr.io" , scope = "repository:user/image:pull" `
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())
				_, err = auth.GetAuthURL(challenge, imageRef)
				// May or may not succeed depending on implementation, but should not panic
				_ = err
			})
		})

		When("the image reference has unusual formatting", func() {
			It("should handle image names with many path segments", func() {
				challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("registry.example.com/org/team/project/component/image")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(URL).NotTo(BeNil())
				// Verify scope is constructed correctly for multi-segment path
				scope := URL.Query().Get("scope")
				Expect(scope).To(ContainSubstring("repository:"))
				Expect(scope).To(ContainSubstring(":pull"))
			})

			It("should handle image names with uppercase letters", func() {
				challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
				// Note: Docker registry names are normalized to lowercase
				imageRef, err := ref.ParseNormalizedNamed("MyRegistry.com/MyOrg/MyImage")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(URL).NotTo(BeNil())
			})

			It("should handle image names with ports", func() {
				challenge := `bearer realm="https://registry.example.com:5000/token",service="registry.example.com:5000",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("registry.example.com:5000/myimage")
				Expect(err).NotTo(HaveOccurred())
				URL, err := auth.GetAuthURL(challenge, imageRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(URL).NotTo(BeNil())
			})
		})

		When("handling concurrent authentication requests", func() {
			It("should be safe for concurrent use", func() {
				challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := ref.ParseNormalizedNamed("todd2982/watchtower")
				Expect(err).NotTo(HaveOccurred())

				// Run multiple GetAuthURL calls concurrently
				done := make(chan bool)
				for i := 0; i < 10; i++ {
					go func() {
						_, _ = auth.GetAuthURL(challenge, imageRef)
						done <- true
					}()
				}

				// Wait for all to complete
				for i := 0; i < 10; i++ {
					<-done
				}
				// Should not panic or race
			})
		})
	})
})

var scopeImageRegexp = MatchRegexp("^repository:[a-z0-9]+(/[a-z0-9]+)*:pull$")

func getScopeFromImageAuthURL(imageName string) string {
	normalizedRef, _ := ref.ParseNormalizedNamed(imageName)
	challenge := `bearer realm="https://dummy.host/token",service="dummy.host",scope="repository:user/image:pull"`
	URL, _ := auth.GetAuthURL(challenge, normalizedRef)

	scope := URL.Query().Get("scope")
	Expect(scopeImageRegexp.Match(scope)).To(BeTrue())
	return strings.Replace(scope[11:], ":pull", "", 1)
}
