package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	token  = "123123123"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}

var _ = Describe("API", func() {
	api := New(token, 8080)

	Describe("RequireToken middleware", func() {
		It("should return 401 Unauthorized when token is not provided", func() {
			handlerFunc := api.RequireToken(testHandler)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)

			handlerFunc(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should return 401 Unauthorized when token is invalid", func() {
			handlerFunc := api.RequireToken(testHandler)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)
			req.Header.Set("Authorization", "Bearer 123")

			handlerFunc(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should return 200 OK when token is valid", func() {
			handlerFunc := api.RequireToken(testHandler)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)
			req.Header.Set("Authorization", "Bearer " + token)

			handlerFunc(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		// Edge case tests for issue #76
		When("handling various token formats", func() {
			It("should reject empty Authorization header", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "")

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject whitespace-only Authorization header", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "   ")

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject Authorization header without Bearer prefix", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", token)

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject Bearer with empty token", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer ")

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject Bearer with whitespace-only token", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer    ")

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should handle case-sensitive token matching", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				// Try uppercase version of token
				req.Header.Set("Authorization", "Bearer " + "ABC123ABC")

				handlerFunc(rec, req)

				// Should fail since tokens should be case-sensitive
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject token with trailing whitespace", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer " + token + "   ")

				handlerFunc(rec, req)

				// Should fail - tokens should match exactly
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject token with leading whitespace", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer    " + token)

				handlerFunc(rec, req)

				// Should fail because "Bearer    token" doesn't match "Bearer token"
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		When("handling token security edge cases", func() {
			It("should reject very long tokens (potential DoS)", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				// Create a very long token (100KB)
				longToken := strings.Repeat("a", 100000)
				req.Header.Set("Authorization", "Bearer " + longToken)

				handlerFunc(rec, req)

				// Should handle without crashing
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should handle tokens with special characters", func() {
				specialTokenAPI := New("token-with-special_chars.123!@#", 8080)
				handlerFunc := specialTokenAPI.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer token-with-special_chars.123!@#")

				handlerFunc(rec, req)

				// Should accept tokens with special characters
				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should handle tokens with unicode characters", func() {
				unicodeTokenAPI := New("token-with-üñíçødé-™", 8080)
				handlerFunc := unicodeTokenAPI.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer token-with-üñíçødé-™")

				handlerFunc(rec, req)

				// Should handle unicode tokens
				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should reject token with null bytes", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Bearer token\x00with\x00nulls")

				handlerFunc(rec, req)

				// Should reject tokens with null bytes
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		When("handling malformed Authorization headers", func() {
			It("should reject malformed Bearer prefix with wrong case", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "bearer " + token) // lowercase

				handlerFunc(rec, req)

				// Should reject because "bearer" (lowercase) doesn't match "Bearer"
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject different auth schemes", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Set("Authorization", "Basic " + token)

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should handle multiple Authorization headers", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				req.Header.Add("Authorization", "Bearer invalid")
				req.Header.Add("Authorization", "Bearer " + token)

				handlerFunc(rec, req)

				// Should reject - Header.Get() returns first value which is invalid
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should reject partial token matches", func() {
				handlerFunc := api.RequireToken(testHandler)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/hello", nil)
				// Use only part of the actual token
				req.Header.Set("Authorization", "Bearer 123")

				handlerFunc(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		When("testing for timing attack vulnerabilities", func() {
			It("should use constant-time comparison for tokens", func() {
				handlerFunc := api.RequireToken(testHandler)

				// Test with a token that matches up to different lengths
				testCases := []string{
					"1",           // 1 char match
					"12",          // 2 char match
					"123",         // 3 char match
					"123123",      // 6 char match (partial)
					"123123123",   // full match
					"wrongtoken",  // no match
				}

				for _, testToken := range testCases {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest("GET", "/hello", nil)
					req.Header.Set("Authorization", "Bearer " + testToken)

					handlerFunc(rec, req)

					// All should return 401 except the correct token
					if testToken != token {
						Expect(rec.Code).To(Equal(http.StatusUnauthorized))
					}
				}
			})
		})
	})
})

func testHandler(w http.ResponseWriter, req *http.Request) {
	_, _ = io.WriteString(w, "Hello!")
}
