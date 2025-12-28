package sorter

import (
	"sort"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/todd2982/watchtower/internal/actions/mocks"
	"github.com/todd2982/watchtower/pkg/container"
	wt "github.com/todd2982/watchtower/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSorter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sorter Suite")
}

var _ = Describe("ByCreated sorting", func() {
	Describe("the Less function", func() {
		When("both containers have valid timestamps", func() {
			It("should sort older containers before newer ones", func() {
				older := mocks.CreateMockContainer("older", "older", "image", time.Now().Add(-1*time.Hour))
				newer := mocks.CreateMockContainer("newer", "newer", "image", time.Now())

				containers := ByCreated{newer, older}
				Expect(containers.Less(0, 1)).To(BeFalse())
				Expect(containers.Less(1, 0)).To(BeTrue())
			})

			It("should maintain sort order with multiple containers", func() {
				c1 := mocks.CreateMockContainer("c1", "c1", "image", time.Now().Add(-3*time.Hour))
				c2 := mocks.CreateMockContainer("c2", "c2", "image", time.Now().Add(-2*time.Hour))
				c3 := mocks.CreateMockContainer("c3", "c3", "image", time.Now().Add(-1*time.Hour))

				containers := ByCreated{c3, c1, c2}
				sort.Sort(containers)

				Expect(containers[0].Name()).To(Equal("c1"))
				Expect(containers[1].Name()).To(Equal("c2"))
				Expect(containers[2].Name()).To(Equal("c3"))
			})
		})

		When("the first container has an invalid timestamp", func() {
			It("should handle the error by using current time for first container", func() {
				// Create a mock container with an invalid timestamp
				invalidContainer := createMockContainerWithInvalidTimestamp("invalid", "invalid", "image")
				validContainer := mocks.CreateMockContainer("valid", "valid", "image", time.Now().Add(-1*time.Hour))

				containers := ByCreated{invalidContainer, validContainer}

				// Invalid timestamp gets set to time.Now(), which is after the valid old timestamp
				// So invalid (index 0) should be "after" valid (index 1), making Less(0,1) = false
				Expect(containers.Less(0, 1)).To(BeFalse())
			})
		})

		When("the second container has an invalid timestamp", func() {
			It("should handle the error by using current time for second container", func() {
				validContainer := mocks.CreateMockContainer("valid", "valid", "image", time.Now().Add(-1*time.Hour))
				invalidContainer := createMockContainerWithInvalidTimestamp("invalid", "invalid", "image")

				containers := ByCreated{validContainer, invalidContainer}

				// Invalid timestamp gets set to time.Now(), which is after the valid old timestamp
				// So valid (index 0) should be "before" invalid (index 1), making Less(0,1) = true
				Expect(containers.Less(0, 1)).To(BeTrue())
			})
		})

		When("both containers have invalid timestamps", func() {
			It("should handle errors for both containers", func() {
				invalid1 := createMockContainerWithInvalidTimestamp("invalid1", "invalid1", "image")
				invalid2 := createMockContainerWithInvalidTimestamp("invalid2", "invalid2", "image")

				containers := ByCreated{invalid1, invalid2}

				// Both get set to time.Now(), which should be approximately equal
				// The result depends on which executes first, but should not panic
				result := containers.Less(0, 1)
				Expect(result).To(BeAssignableToTypeOf(false))
			})
		})

		When("containers have the same creation time", func() {
			It("should handle equal timestamps correctly", func() {
				now := time.Now()
				c1 := mocks.CreateMockContainer("c1", "c1", "image", now)
				c2 := mocks.CreateMockContainer("c2", "c2", "image", now)

				containers := ByCreated{c1, c2}
				Expect(containers.Less(0, 1)).To(BeFalse())
				Expect(containers.Less(1, 0)).To(BeFalse())
			})
		})

		When("timestamps have nanosecond precision differences", func() {
			It("should correctly compare timestamps with nanosecond precision", func() {
				t1 := time.Now()
				t2 := t1.Add(1 * time.Nanosecond)

				c1 := mocks.CreateMockContainer("c1", "c1", "image", t1)
				c2 := mocks.CreateMockContainer("c2", "c2", "image", t2)

				containers := ByCreated{c1, c2}
				Expect(containers.Less(0, 1)).To(BeTrue())
				Expect(containers.Less(1, 0)).To(BeFalse())
			})
		})
	})

	Describe("the Len function", func() {
		It("should return the correct length", func() {
			containers := ByCreated{
				mocks.CreateMockContainer("c1", "c1", "image", time.Now()),
				mocks.CreateMockContainer("c2", "c2", "image", time.Now()),
				mocks.CreateMockContainer("c3", "c3", "image", time.Now()),
			}
			Expect(containers.Len()).To(Equal(3))
		})

		It("should return 0 for empty slice", func() {
			containers := ByCreated{}
			Expect(containers.Len()).To(Equal(0))
		})
	})

	Describe("the Swap function", func() {
		It("should swap two containers", func() {
			c1 := mocks.CreateMockContainer("c1", "c1", "image", time.Now())
			c2 := mocks.CreateMockContainer("c2", "c2", "image", time.Now())

			containers := ByCreated{c1, c2}
			containers.Swap(0, 1)

			Expect(containers[0].Name()).To(Equal("c2"))
			Expect(containers[1].Name()).To(Equal("c1"))
		})
	})
})

var _ = Describe("SortByDependencies", func() {
	When("containers have no dependencies", func() {
		It("should return containers in original order", func() {
			c1 := mocks.CreateMockContainerWithLinks("c1", "c1", "image", time.Now(), []string{}, nil)
			c2 := mocks.CreateMockContainerWithLinks("c2", "c2", "image", time.Now(), []string{}, nil)

			result, err := SortByDependencies([]wt.Container{c1, c2})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})
	})

	When("containers have circular dependencies", func() {
		It("should return an error", func() {
			// Container c1 depends on c2, and c2 depends on c1 (circular)
			c1 := mocks.CreateMockContainerWithLinks("c1", "c1", "image", time.Now(), []string{"c2"}, nil)
			c2 := mocks.CreateMockContainerWithLinks("c2", "c2", "image", time.Now(), []string{"c1"}, nil)

			_, err := SortByDependencies([]wt.Container{c1, c2})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("circular reference"))
		})
	})

	When("containers have linear dependencies", func() {
		It("should sort them in dependency order", func() {
			// c3 depends on c2, c2 depends on c1
			c1 := mocks.CreateMockContainerWithLinks("c1", "c1", "image", time.Now(), []string{}, nil)
			c2 := mocks.CreateMockContainerWithLinks("c2", "c2", "image", time.Now(), []string{"c1"}, nil)
			c3 := mocks.CreateMockContainerWithLinks("c3", "c3", "image", time.Now(), []string{"c2"}, nil)

			result, err := SortByDependencies([]wt.Container{c3, c2, c1})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(3))
			// c1 should be first (no dependencies)
			Expect(result[0].Name()).To(Equal("c1"))
			// c2 should be second (depends on c1)
			Expect(result[1].Name()).To(Equal("c2"))
			// c3 should be last (depends on c2)
			Expect(result[2].Name()).To(Equal("c3"))
		})
	})

	When("given an empty list", func() {
		It("should return an empty list", func() {
			result, err := SortByDependencies([]wt.Container{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})
	})
})

// Helper function to create a mock container with an invalid timestamp
func createMockContainerWithInvalidTimestamp(id, name, image string) wt.Container {
	// We need to create a container with an invalid Created timestamp
	// The mocks package formats timestamps properly, so we need to work around that
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      id,
			Image:   image,
			Name:    name,
			Created: "invalid-timestamp-format",
		},
		Config: &dockerContainer.Config{
			Image:  image,
			Labels: make(map[string]string),
		},
	}
	return container.NewContainer(
		&content,
		mocks.CreateMockImageInfo(image),
	)
}
