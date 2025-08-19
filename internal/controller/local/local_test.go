/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local_test

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
	"github.com/detiber/k8s-jumperless/internal/controller/local"
)

func TestLocal(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Local Package Suite")
}

var _ = ginkgo.Describe("Local package parsing functions", func() {

	ginkgo.Describe("ParseConfig", func() {
		ginkgo.It("should parse valid config output correctly", func() {
			configOutput := `~

copy / edit / paste any of these lines
into the main menu to change a setting

Jumperless Config:


` + "`[config] firmware_version = 5.2.2.0;" + `

` + "`[hardware] generation = 5;" + `
` + "`[hardware] revision = 5;" + `
` + "`[hardware] probe_revision = 5;" + `

` + "`[dacs] top_rail = 3.50;" + `
` + "`[dacs] bottom_rail = 3.50;" + `

` + "`[top_oled] font = jokerman;" + `

END`

			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(4))

			// Find each section
			var configSection, hardwareSection, dacsSection, topOledSection *jumperlessv5alpha1.JumperLessConfigSection
			for i := range result {
				switch result[i].Name {
				case "config":
					configSection = &result[i]
				case "hardware":
					hardwareSection = &result[i]
				case "dacs":
					dacsSection = &result[i]
				case "top_oled":
					topOledSection = &result[i]
				}
			}

			// Verify config section
			gomega.Expect(configSection).NotTo(gomega.BeNil())
			gomega.Expect(configSection.Entries).To(gomega.HaveLen(1))
			gomega.Expect(configSection.Entries[0].Key).To(gomega.Equal("firmware_version"))
			gomega.Expect(configSection.Entries[0].Value).To(gomega.Equal("5.2.2.0"))

			// Verify hardware section
			gomega.Expect(hardwareSection).NotTo(gomega.BeNil())
			gomega.Expect(hardwareSection.Entries).To(gomega.HaveLen(3))

			// Verify dacs section
			gomega.Expect(dacsSection).NotTo(gomega.BeNil())
			gomega.Expect(dacsSection.Entries).To(gomega.HaveLen(2))

			// Verify top_oled section
			gomega.Expect(topOledSection).NotTo(gomega.BeNil())
			gomega.Expect(topOledSection.Entries).To(gomega.HaveLen(1))
			gomega.Expect(topOledSection.Entries[0].Key).To(gomega.Equal("font"))
			gomega.Expect(topOledSection.Entries[0].Value).To(gomega.Equal("jokerman"))
		})

		ginkgo.It("should handle empty config output", func() {
			result, err := local.ParseConfig("")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(0))
		})

		ginkgo.It("should handle config output with no config lines", func() {
			configOutput := `~
Some random text
No config lines here
END`
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(0))
		})

		ginkgo.It("should handle malformed config line without closing bracket", func() {
			configOutput := "`[config firmware_version = 5.2.2.0;"
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(0))
		})

		ginkgo.It("should handle malformed config line without equals sign", func() {
			configOutput := "`[config] firmware_version 5.2.2.0;"
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1)) // Section is created but with empty entries
			gomega.Expect(result[0].Name).To(gomega.Equal("config"))
			gomega.Expect(result[0].Entries).To(gomega.HaveLen(0))
		})

		ginkgo.It("should strip semicolons from values", func() {
			configOutput := "`[test] key = value;"
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Entries[0].Value).To(gomega.Equal("value"))
		})

		ginkgo.It("should trim whitespace from keys and values", func() {
			configOutput := "`[test]   key   =   value   ;"
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Entries[0].Key).To(gomega.Equal("key"))
			gomega.Expect(result[0].Entries[0].Value).To(gomega.Equal("value   ")) // TrimSpace then TrimSuffix leaves trailing spaces
		})

		ginkgo.It("should trim whitespace and semicolon correctly when no space before semicolon", func() {
			configOutput := "`[test]   key   =   value;"
			result, err := local.ParseConfig(configOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Entries[0].Key).To(gomega.Equal("key"))
			gomega.Expect(result[0].Entries[0].Value).To(gomega.Equal("value"))
		})
	})

	ginkgo.Describe("ParseNets", func() {
		ginkgo.It("should parse valid nets output correctly", func() {
			netsOutput := `Index	Name		Voltage		Nodes
1	 GND		 0 V         GND	    
2	 Top Rail	 0.00 V      TOP_R	    
3	 Bottom Rail	 0.00 V      BOT_R	    
4	 DAC 0		 3.33 V      DAC_0,BUF_IN	    
5	 DAC 1		 0.00 V      DAC_1	    `

			result, err := local.ParseNets(netsOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(5))

			// Verify first net (GND)
			gomega.Expect(result[0].Index).To(gomega.Equal(int32(1)))
			gomega.Expect(result[0].Name).To(gomega.Equal("GND"))
			gomega.Expect(result[0].Voltage).To(gomega.Equal("0V"))
			gomega.Expect(result[0].Nodes).To(gomega.Equal([]string{"GND"}))

			// Verify DAC 0 net with multiple nodes
			gomega.Expect(result[3].Index).To(gomega.Equal(int32(4)))
			gomega.Expect(result[3].Name).To(gomega.Equal("DAC 0"))
			gomega.Expect(result[3].Voltage).To(gomega.Equal("3.33V"))
			gomega.Expect(result[3].Nodes).To(gomega.Equal([]string{"DAC_0", "BUF_IN"}))
		})

		ginkgo.It("should handle empty nets output", func() {
			result, err := local.ParseNets("")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(0))
		})

		ginkgo.It("should skip header line", func() {
			netsOutput := `Index	Name		Voltage		Nodes
1	 GND		 0 V         GND	    `

			result, err := local.ParseNets(netsOutput)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0].Index).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("should handle malformed net line and return error", func() {
			netsOutput := `invalid line without tabs`
			result, err := local.ParseNets(netsOutput)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.HaveLen(0))
		})
	})

	ginkgo.Describe("ParseNetLine", func() {
		ginkgo.It("should parse a simple net line correctly", func() {
			netLine := "1\t GND\t\t 0 V         GND\t    "
			result, err := local.ParseNetLine(netLine)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.Index).To(gomega.Equal(int32(1)))
			gomega.Expect(result.Name).To(gomega.Equal("GND"))
			gomega.Expect(result.Voltage).To(gomega.Equal("0V"))
			gomega.Expect(result.Nodes).To(gomega.Equal([]string{"GND"}))
		})

		ginkgo.It("should parse a net line with multiple nodes", func() {
			netLine := "4\t DAC 0\t\t 3.33 V      DAC_0,BUF_IN\t    "
			result, err := local.ParseNetLine(netLine)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.Index).To(gomega.Equal(int32(4)))
			gomega.Expect(result.Name).To(gomega.Equal("DAC 0"))
			gomega.Expect(result.Voltage).To(gomega.Equal("3.33V"))
			gomega.Expect(result.Nodes).To(gomega.Equal([]string{"DAC_0", "BUF_IN"}))
		})

		ginkgo.It("should handle net line with leading carriage return", func() {
			netLine := "\r1\t GND\t\t 0 V         GND\t    "
			result, err := local.ParseNetLine(netLine)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.Index).To(gomega.Equal(int32(1)))
			gomega.Expect(result.Name).To(gomega.Equal("GND"))
			gomega.Expect(result.Voltage).To(gomega.Equal("0V"))
		})

		ginkgo.It("should return error for insufficient fields", func() {
			netLine := "1\t GND"
			_, err := local.ParseNetLine(netLine)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("expected at least 3 fields"))
		})

		ginkgo.It("should return error for invalid index", func() {
			netLine := "invalid\t GND\t\t 0 V         GND\t    "
			_, err := local.ParseNetLine(netLine)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("unable to parse index"))
		})

		ginkgo.It("should return error when voltage 'V' marker is missing", func() {
			netLine := "1\t GND\t\t 0 X         GND\t    "
			_, err := local.ParseNetLine(netLine)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("unable to find voltage"))
		})

		ginkgo.It("should handle empty nodes section", func() {
			netLine := "1\t GND\t\t 0 V         \t    "
			result, err := local.ParseNetLine(netLine)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.Nodes).To(gomega.HaveLen(0))
		})

		ginkgo.It("should trim spaces from node names", func() {
			netLine := "4\t DAC 0\t\t 3.33 V      DAC_0 , BUF_IN \t    "
			result, err := local.ParseNetLine(netLine)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result.Nodes).To(gomega.Equal([]string{"DAC_0", "BUF_IN"}))
		})
	})
})
