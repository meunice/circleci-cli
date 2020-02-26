package local

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("build", func() {

	Describe("config compilation", func() {
		It("works", func() {
			Expect(extractConfigPath([]string{"-c", "b"})).To(
				Equal(&processedArgs{
					configPath: "b",
					args:       []string{},
				}))

			Expect(extractConfigPath([]string{})).To(
				Equal(&processedArgs{
					configPath: ".circleci/config.yml",
					args:       []string{},
				}))

			Expect(extractConfigPath([]string{"a", "b", "--config", "foo", "d"})).To(
				Equal(&processedArgs{
					configPath: "foo",
					args:       []string{"a", "b", "d"},
				}))

			_, err := extractConfigPath([]string{"a", "b", "--config"})
			Expect(err).To(MatchError("flag needs an argument: --config"))
		})
	})

	Describe("loading settings", func() {

		var (
			tempHome string
		)

		BeforeEach(func() {
			var err error
			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")

			Expect(err).ToNot(HaveOccurred())
			Expect(os.Setenv("HOME", tempHome)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("can load settings", func() {
			Expect(storeBuildAgentSha("deipnosophist")).To(Succeed())
			Expect(loadCurrentBuildAgentSha()).To(Equal("deipnosophist"))
			image, err := picardImage()
			Expect(err).ToNot(HaveOccurred())
			Expect(image).To(Equal("circleci/picard@deipnosophist"))
		})

	})
})
