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

package local

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	jumperlessv5alpha1 "github.com/detiber/k8s-jumperless/api/v5alpha1"
)

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Package Suite")
}

var _ = Describe("Local package parsing functions", func() {

	Describe("parseConfig", func() {
		It("should parse valid config output correctly", func() {
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

			result, err := parseConfig(configOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(4))

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
			Expect(configSection).NotTo(BeNil())
			Expect(configSection.Entries).To(HaveLen(1))
			Expect(configSection.Entries[0].Key).To(Equal("firmware_version"))
			Expect(configSection.Entries[0].Value).To(Equal("5.2.2.0"))

			// Verify hardware section
			Expect(hardwareSection).NotTo(BeNil())
			Expect(hardwareSection.Entries).To(HaveLen(3))

			// Verify dacs section
			Expect(dacsSection).NotTo(BeNil())
			Expect(dacsSection.Entries).To(HaveLen(2))

			// Verify top_oled section
			Expect(topOledSection).NotTo(BeNil())
			Expect(topOledSection.Entries).To(HaveLen(1))
			Expect(topOledSection.Entries[0].Key).To(Equal("font"))
			Expect(topOledSection.Entries[0].Value).To(Equal("jokerman"))
		})

		It("should handle empty config output", func() {
			result, err := parseConfig("")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})

		It("should handle config output with no config lines", func() {
			configOutput := `~
Some random text
No config lines here
END`
			result, err := parseConfig(configOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})

		It("should handle malformed config line without closing bracket", func() {
			configOutput := "`[config firmware_version = 5.2.2.0;"
			result, err := parseConfig(configOutput)
			Expect(err).To(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})

		It("should handle malformed config line without equals sign", func() {
			configOutput := "`[config] firmware_version 5.2.2.0;"
			result, err := parseConfig(configOutput)
			Expect(err).To(HaveOccurred())
			Expect(result).To(HaveLen(1)) // Section is created but with empty entries
			Expect(result[0].Name).To(Equal("config"))
			Expect(result[0].Entries).To(HaveLen(0))
		})

		It("should strip semicolons from values", func() {
			configOutput := "`[test] key = value;"
			result, err := parseConfig(configOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Entries[0].Value).To(Equal("value"))
		})

		It("should trim whitespace from keys and values", func() {
			configOutput := "`[test]   key   =   value   ;"
			result, err := parseConfig(configOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Entries[0].Key).To(Equal("key"))
			Expect(result[0].Entries[0].Value).To(Equal("value   ")) // TrimSpace then TrimSuffix leaves trailing spaces
		})

		It("should trim whitespace and semicolon correctly when no space before semicolon", func() {
			configOutput := "`[test]   key   =   value;"
			result, err := parseConfig(configOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Entries[0].Key).To(Equal("key"))
			Expect(result[0].Entries[0].Value).To(Equal("value"))
		})
	})

	Describe("parseNets", func() {
		It("should parse valid nets output correctly", func() {
			netsOutput := `Index	Name		Voltage		Nodes
1	 GND		 0 V         GND	    
2	 Top Rail	 0.00 V      TOP_R	    
3	 Bottom Rail	 0.00 V      BOT_R	    
4	 DAC 0		 3.33 V      DAC_0,BUF_IN	    
5	 DAC 1		 0.00 V      DAC_1	    `

			result, err := parseNets(netsOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(5))

			// Verify first net (GND)
			Expect(result[0].Index).To(Equal(int32(1)))
			Expect(result[0].Name).To(Equal("GND"))
			Expect(result[0].Voltage).To(Equal("0V"))
			Expect(result[0].Nodes).To(Equal([]string{"GND"}))

			// Verify DAC 0 net with multiple nodes
			Expect(result[3].Index).To(Equal(int32(4)))
			Expect(result[3].Name).To(Equal("DAC 0"))
			Expect(result[3].Voltage).To(Equal("3.33V"))
			Expect(result[3].Nodes).To(Equal([]string{"DAC_0", "BUF_IN"}))
		})

		It("should handle empty nets output", func() {
			result, err := parseNets("")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})

		It("should skip header line", func() {
			netsOutput := `Index	Name		Voltage		Nodes
1	 GND		 0 V         GND	    `

			result, err := parseNets(netsOutput)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Index).To(Equal(int32(1)))
		})

		It("should handle malformed net line and return error", func() {
			netsOutput := `invalid line without tabs`
			result, err := parseNets(netsOutput)
			Expect(err).To(HaveOccurred())
			Expect(result).To(HaveLen(0))
		})
	})

	Describe("parseNetLine", func() {
		It("should parse a simple net line correctly", func() {
			netLine := "1\t GND\t\t 0 V         GND\t    "
			result, err := parseNetLine(netLine)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Index).To(Equal(int32(1)))
			Expect(result.Name).To(Equal("GND"))
			Expect(result.Voltage).To(Equal("0V"))
			Expect(result.Nodes).To(Equal([]string{"GND"}))
		})

		It("should parse a net line with multiple nodes", func() {
			netLine := "4\t DAC 0\t\t 3.33 V      DAC_0,BUF_IN\t    "
			result, err := parseNetLine(netLine)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Index).To(Equal(int32(4)))
			Expect(result.Name).To(Equal("DAC 0"))
			Expect(result.Voltage).To(Equal("3.33V"))
			Expect(result.Nodes).To(Equal([]string{"DAC_0", "BUF_IN"}))
		})

		It("should handle net line with leading carriage return", func() {
			netLine := "\r1\t GND\t\t 0 V         GND\t    "
			result, err := parseNetLine(netLine)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Index).To(Equal(int32(1)))
			Expect(result.Name).To(Equal("GND"))
			Expect(result.Voltage).To(Equal("0V"))
		})

		It("should return error for insufficient fields", func() {
			netLine := "1\t GND"
			_, err := parseNetLine(netLine)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected at least 3 fields"))
		})

		It("should return error for invalid index", func() {
			netLine := "invalid\t GND\t\t 0 V         GND\t    "
			_, err := parseNetLine(netLine)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to parse index"))
		})

		It("should return error when voltage 'V' marker is missing", func() {
			netLine := "1\t GND\t\t 0 X         GND\t    "
			_, err := parseNetLine(netLine)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to find voltage"))
		})

		It("should handle empty nodes section", func() {
			netLine := "1\t GND\t\t 0 V         \t    "
			result, err := parseNetLine(netLine)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Nodes).To(HaveLen(0))
		})

		It("should trim spaces from node names", func() {
			netLine := "4\t DAC 0\t\t 3.33 V      DAC_0 , BUF_IN \t    "
			result, err := parseNetLine(netLine)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Nodes).To(Equal([]string{"DAC_0", "BUF_IN"}))
		})
	})
})
